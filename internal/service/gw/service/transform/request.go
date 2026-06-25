package transform

import (
	"context"

	"github.com/dop251/goja"
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
// the incoming value". Path is intentionally not overridable: routing to the
// backend (app prefix + custom path) stays the gateway's responsibility.
type Result struct {
	Method  *string
	Headers map[string][]string
	Params  map[string][]string
	Body    []byte
	BodySet bool
}

type RequestTransformer struct{ r *runner }

func NewRequest(script string, maxWorkers int) (*RequestTransformer, error) {
	r, err := newRunner(wrapRequest(script), maxWorkers)
	if err != nil {
		return nil, err
	}
	return &RequestTransformer{r: r}, nil
}

func (t *RequestTransformer) Transform(ctx context.Context, in *Request) (*Result, error) {
	res := &Result{}
	err := t.r.run(ctx,
		func(rt *goja.Runtime) goja.Value {
			o := rt.NewObject()
			_ = o.Set("method", in.Method)
			_ = o.Set("path", in.Path)
			_ = o.Set("headers", newStrListObject(rt, in.Headers))
			_ = o.Set("params", newStrListObject(rt, in.Params))
			_ = o.Set("vars", newStrObject(rt, in.Vars))
			_ = o.Set("raw_body", string(in.Body))
			return o
		},
		func(rt *goja.Runtime, out goja.Value) {
			if !present(out) {
				return
			}
			obj := out.ToObject(rt)
			if mv := obj.Get("method"); present(mv) {
				s := mv.String()
				res.Method = &s
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
		},
	)
	if err != nil {
		return nil, err
	}
	return res, nil
}

// wrap embeds the user script in a function that parses the body, runs the
// script, and normalizes its return into the shape parseResult expects. Bad
// return values throw here, surfacing as a run error so the gateway fails the
// request without calling the backend.
func wrapRequest(script string) string {
	return `(function (req) {
  "use strict";
` + strlistmapJS + `
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
  if ("headers" in out) res.headers = __strlistmap(out.headers, "headers");
  if ("params" in out) res.params = __strlistmap(out.params, "params");
  if ("body" in out) ` + bodyOutJS("res", "out.body") + `
  return res;
})`
}
