// Package transform evaluates per-endpoint JavaScript that reshapes traffic at
// the gateway: a request before it is proxied to the backend (see request.go),
// or a response before it is returned to the client (see response.go).
//
// A script is compiled once (when the gateway rebuilds its handlers from a new
// snapshot) and executed per call on a bounded pool of goja runtimes. The pool
// size (maxWorkers) hard-caps the number of live runtimes — and therefore the
// memory the script can use under load: at most maxWorkers runtimes exist at
// once, and calls beyond that wait for a free slot (bounded by their context).
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
	// concurrently when the snapshot does not specify a limit.
	DefaultMaxWorkers = 16
)

// vm is a runtime paired with the wrapper function evaluated in it. Runtimes are
// not safe for concurrent use, so each is borrowed for a single run.
type vm struct {
	rt  *goja.Runtime
	fn  goja.Callable
	err error
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

// runner compiles one wrapped script and runs it on a bounded pool. It is the
// shared core behind RequestTransformer and ResponseTransformer.
type runner struct {
	prog    *goja.Program
	timeout time.Duration
	tokens  chan struct{} // capacity maxWorkers; a token is the right to run
	free    chan *vm      // capacity maxWorkers; idle runtimes ready for reuse
}

func newRunner(wrappedSrc string, maxWorkers int) (*runner, error) {
	if maxWorkers <= 0 {
		maxWorkers = DefaultMaxWorkers
	}

	prog, err := goja.Compile("transform.js", wrappedSrc, true)
	if err != nil {
		return nil, fmt.Errorf("compile: %w", err)
	}

	r := &runner{
		prog:    prog,
		timeout: defaultTimeout,
		tokens:  make(chan struct{}, maxWorkers),
		free:    make(chan *vm, maxWorkers),
	}
	// Runtimes are created lazily on demand, so an idle endpoint holds none.
	for range maxWorkers {
		r.tokens <- struct{}{}
	}
	return r, nil
}

// run borrows a runtime, builds the input argument, executes the script, and
// parses its return value — all while holding a single pooled runtime. build and
// parse must only touch the passed runtime, never escape values from it.
func (r *runner) run(
	ctx context.Context,
	build func(rt *goja.Runtime) goja.Value,
	parse func(rt *goja.Runtime, out goja.Value),
) error {
	// Acquire a worker slot. Above maxWorkers concurrent calls this blocks until
	// one frees up, applying backpressure instead of allocating more runtimes.
	select {
	case <-r.tokens:
	case <-ctx.Done():
		return ctx.Err()
	}
	defer func() { r.tokens <- struct{}{} }()

	var e *vm
	select {
	case e = <-r.free:
	default:
		e = newVM(r.prog)
	}
	if e.err != nil {
		return e.err
	}

	// Only a healthy runtime is returned to the pool; one that errored (script
	// throw, timeout interrupt) is dropped. This defer runs before the token
	// release above (LIFO), so the runtime is back in free before another
	// goroutine can claim the slot.
	keep := false
	defer func() {
		if keep {
			r.free <- e
		}
	}()

	rt := e.rt
	arg := build(rt)

	timer := time.AfterFunc(r.timeout, func() { rt.Interrupt("timeout") })
	out, runErr := e.fn(goja.Undefined(), arg)
	timer.Stop()
	rt.ClearInterrupt()
	if runErr != nil {
		return fmt.Errorf("run: %w", runErr)
	}

	parse(rt, out)
	keep = true
	return nil
}

// --- shared goja helpers ---

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

// strlistmapJS coerces a returned headers/params object into {key: [string,...]}.
// Shared verbatim by the request and response wrappers.
const strlistmapJS = `
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
        out[k] = [String(val)];
      }
    }
    return out;
  }
`

// bodyOutJS normalizes a returned body into res.body (string) + res.body_set.
// outVar is the result object name, srcExpr the JS expression for out.body.
func bodyOutJS(resVar, srcExpr string) string {
	return `
  {
    var __b = ` + srcExpr + `;
    if (__b === undefined || __b === null) {
      ` + resVar + `.body = "";
    } else if (typeof __b === "string") {
      ` + resVar + `.body = __b;
    } else {
      ` + resVar + `.body = JSON.stringify(__b);
    }
    ` + resVar + `.body_set = true;
  }
`
}
