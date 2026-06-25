// Package transform evaluates a per-endpoint JavaScript that reshapes an
// incoming request before it is proxied to the backend.
//
// A script is compiled once (when the gateway rebuilds its handlers from a new
// snapshot) and executed per request on a pooled, non-shared *goja.Runtime.
//
// Contract exposed to the script:
//
//	in:  req {method, path, headers, params, body, raw_body, vars}
//	     - headers/params are multi-value: {key: [str, ...]} (like http.Header)
//	     - req.body is req.raw_body parsed as JSON (undefined if empty/not JSON)
//	out: return an object with any subset of {method, path, headers, params, body}
//	     - a field that is absent is left as-is (the gateway proxies it unchanged)
//	     - body: object -> JSON.stringify; string -> used as-is; null -> empty body
//	     - headers/params: when returned, replace the whole set (spread req.* to keep);
//	       each value may be a list or a bare string (coerced to a single-value list)
package transform

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dop251/goja"
)

const (
	defaultTimeout = 100 * time.Millisecond

	// DefaultMaxWorkers caps how many goja runtimes a single script may hold
	// concurrently when the snapshot does not specify a limit. It bounds memory
	// at the cost of queueing requests above the cap.
	DefaultMaxWorkers = 16
)

// Request is the normalized incoming request handed to the script as `req`.
// Headers and Params are multi-value, matching http.Header / url.Values.
type Request struct {
	Method  string
	Path    string
	Headers map[string][]string
	Params  map[string][]string
	Body    []byte
	Vars    map[string]string
}

// Result is what the script asked the gateway to send to the backend. A nil
// pointer / nil map / BodySet=false means "the script did not touch this — keep
// the incoming value".
type Result struct {
	Method  *string
	Path    *string
	Headers map[string][]string
	Params  map[string][]string
	Body    []byte
	BodySet bool
}

// Transformer runs one script on a bounded pool. The pool size (maxWorkers)
// hard-caps the number of live goja runtimes, and therefore the memory the
// script can use under load: at most maxWorkers runtimes exist at once, and
// requests beyond that wait for a free slot (bounded by their context).
type Transformer struct {
	prog    *goja.Program
	timeout time.Duration
	tokens  chan struct{} // capacity maxWorkers; a token is the right to run
	free    chan *vm      // capacity maxWorkers; idle runtimes ready for reuse
}

// vm is a runtime paired with the wrapper function evaluated in it. Runtimes are
// not safe for concurrent use, so each is borrowed for a single Transform call.
type vm struct {
	rt  *goja.Runtime
	fn  goja.Callable
	err error
}

func New(script string, maxWorkers int) (*Transformer, error) {
	if maxWorkers <= 0 {
		maxWorkers = DefaultMaxWorkers
	}

	prog, err := goja.Compile("transform.js", wrap(script), true)
	if err != nil {
		return nil, fmt.Errorf("compile: %w", err)
	}

	t := &Transformer{
		prog:    prog,
		timeout: defaultTimeout,
		tokens:  make(chan struct{}, maxWorkers),
		free:    make(chan *vm, maxWorkers),
	}
	// Runtimes are created lazily on demand, so an idle endpoint holds none.
	for range maxWorkers {
		t.tokens <- struct{}{}
	}
	return t, nil
}

func newVM(prog *goja.Program) *vm {
	rt := goja.New()
	v, err := rt.RunProgram(prog)
	if err != nil {
		return &vm{err: fmt.Errorf("init: %w", err)}
	}
	fn, ok := goja.AssertFunction(v)
	if !ok {
		return &vm{err: errors.New("init: wrapper is not callable")}
	}
	return &vm{rt: rt, fn: fn}
}

func (t *Transformer) Transform(ctx context.Context, in *Request) (_ *Result, finalErr error) {
	// Acquire a worker slot. Above maxWorkers concurrent calls this blocks until
	// one frees up, applying backpressure instead of allocating more runtimes.
	select {
	case <-t.tokens:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	defer func() { t.tokens <- struct{}{} }()

	// Reuse an idle runtime or lazily create one (never exceeds maxWorkers,
	// since we hold a token).
	var e *vm
	select {
	case e = <-t.free:
	default:
		e = newVM(t.prog)
	}
	if e.err != nil {
		return nil, e.err
	}

	// Only return a healthy runtime to the pool. A runtime that errored (script
	// throw, timeout interrupt) is dropped so the next call gets a clean one.
	// This defer runs before the token release above (LIFO), so the runtime is
	// back in free before another goroutine can claim the slot.
	keep := false
	defer func() {
		if keep {
			t.free <- e
		}
	}()

	rt := e.rt

	reqObj := rt.NewObject()
	_ = reqObj.Set("method", in.Method)
	_ = reqObj.Set("path", in.Path)
	_ = reqObj.Set("headers", newStrListObject(rt, in.Headers))
	_ = reqObj.Set("params", newStrListObject(rt, in.Params))
	_ = reqObj.Set("vars", newStrObject(rt, in.Vars))
	_ = reqObj.Set("raw_body", string(in.Body))

	timer := time.AfterFunc(t.timeout, func() { rt.Interrupt("timeout") })
	v, runErr := e.fn(goja.Undefined(), reqObj)
	timer.Stop()
	rt.ClearInterrupt()
	if runErr != nil {
		return nil, fmt.Errorf("run: %w", runErr)
	}

	res, err := parseResult(rt, v)
	if err != nil {
		return nil, err
	}

	keep = true
	return res, nil
}

func parseResult(rt *goja.Runtime, v goja.Value) (*Result, error) {
	res := &Result{}
	if v == nil || goja.IsUndefined(v) || goja.IsNull(v) {
		return res, nil
	}

	obj := v.ToObject(rt)

	if mv := obj.Get("method"); present(mv) {
		s := mv.String()
		res.Method = &s
	}
	if pv := obj.Get("path"); present(pv) {
		s := pv.String()
		res.Path = &s
	}
	if hv := obj.Get("headers"); present(hv) {
		res.Headers = exportStrListMap(hv)
	}
	if pv := obj.Get("params"); present(pv) {
		res.Params = exportStrListMap(pv)
	}
	if bsv := obj.Get("body_set"); bsv != nil && bsv.ToBoolean() {
		res.BodySet = true
		res.Body = []byte(obj.Get("body").String())
	}

	return res, nil
}

func present(v goja.Value) bool {
	return v != nil && !goja.IsUndefined(v) && !goja.IsNull(v)
}

// exportStrListMap reads an object the wrapper already coerced to {key: [str,...]}.
func exportStrListMap(v goja.Value) map[string][]string {
	exp, ok := v.Export().(map[string]any)
	if !ok {
		return map[string][]string{}
	}
	out := make(map[string][]string, len(exp))
	for k, val := range exp {
		arr, ok := val.([]any)
		if !ok {
			continue
		}
		list := make([]string, len(arr))
		for i, item := range arr {
			list[i] = fmt.Sprint(item)
		}
		out[k] = list
	}
	return out
}

func newStrObject(rt *goja.Runtime, m map[string]string) *goja.Object {
	o := rt.NewObject()
	for k, v := range m {
		_ = o.Set(k, v)
	}
	return o
}

// newStrListObject builds a native JS object whose values are native JS arrays,
// so the script can spread, index, and Array.isArray them without surprises.
func newStrListObject(rt *goja.Runtime, m map[string][]string) *goja.Object {
	o := rt.NewObject()
	for k, vs := range m {
		items := make([]any, len(vs))
		for i, v := range vs {
			items[i] = v
		}
		_ = o.Set(k, rt.NewArray(items...))
	}
	return o
}

// wrap embeds the user script in a function that parses the body, runs the
// script, and normalizes its return value into the shape parseResult expects.
// Bad return values (non-object, non-object headers/params) throw here, which
// surfaces as a run error — so the gateway fails the request without calling the
// backend.
func wrap(script string) string {
	return `(function (req) {
  "use strict";

  function __strlistmap(v, name) {
    if (v === null || v === undefined) return {};
    if (typeof v !== "object") throw new Error(name + " must be an object");
    var out = {};
    for (var k in v) {
      if (!Object.prototype.hasOwnProperty.call(v, k)) continue;
      var val = v[k];
      if (val === null || val === undefined) {
        out[k] = [];
      } else if (Array.isArray(val)) {
        out[k] = val.map(function (x) {
          return (x === null || x === undefined) ? "" : String(x);
        });
      } else {
        // scalar -> single-value list, for ergonomics
        out[k] = [String(val)];
      }
    }
    return out;
  }

  req.body = (function () {
    try {
      return (req.raw_body && req.raw_body.length) ? JSON.parse(req.raw_body) : undefined;
    } catch (e) {
      return undefined;
    }
  })();

  var out = (function (req) {
` + script + `
  })(req);

  if (out === undefined || out === null) return {};
  if (typeof out !== "object") throw new Error("script must return an object");

  var res = {};
  if ("method" in out) res.method = String(out.method);
  if ("path" in out) res.path = String(out.path);
  if ("headers" in out) res.headers = __strlistmap(out.headers, "headers");
  if ("params" in out) res.params = __strlistmap(out.params, "params");
  if ("body" in out) {
    var b = out.body;
    if (b === undefined || b === null) {
      res.body = "";
    } else if (typeof b === "string") {
      res.body = b;
    } else {
      res.body = JSON.stringify(b);
    }
    res.body_set = true;
  }
  return res;
})`
}
