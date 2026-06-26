package transform

import (
	"context"

	"github.com/dop251/goja"
)

// Response is the normalized backend response handed to the script as `res`.
type Response struct {
	Status  int
	Headers map[string][]string
	Body    []byte
	Vars    map[string]string
}

// ResponseResult is what the script asked the gateway to send to the client. A
// nil pointer / nil map / BodySet=false means "the script did not touch this —
// keep the backend's value".
type ResponseResult struct {
	Status  *int
	Headers map[string][]string
	Body    []byte
	BodySet bool
}

type ResponseTransformer struct{ r *runner }

func NewResponse(script string, maxWorkers int) (*ResponseTransformer, error) {
	r, err := newRunner(wrapResponse(script), maxWorkers)
	if err != nil {
		return nil, err
	}
	return &ResponseTransformer{r: r}, nil
}

func (t *ResponseTransformer) Transform(ctx context.Context, in *Response) (*ResponseResult, error) {
	res := &ResponseResult{}
	err := t.r.run(ctx,
		func(rt *goja.Runtime) goja.Value {
			o := rt.NewObject()
			_ = o.Set("status", in.Status)
			_ = o.Set("headers", newStrListObject(rt, in.Headers))
			_ = o.Set("vars", newStrObject(rt, in.Vars))
			_ = o.Set("raw_body", string(in.Body))
			return o
		},
		func(rt *goja.Runtime, out goja.Value) {
			if !present(out) {
				return
			}
			obj := out.ToObject(rt)
			if sv := obj.Get("status"); present(sv) {
				res.Status = new(int(sv.ToInteger()))
			}
			if hv := obj.Get("headers"); present(hv) {
				res.Headers = exportStrListMap(hv)
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

func wrapResponse(script string) string {
	return `(function (res) {
  "use strict";
` + strlistmapJS + `
  res.body = (function () {
    try {
      return (res.raw_body && res.raw_body.length) ? JSON.parse(res.raw_body) : undefined;
    } catch (e) {
      return undefined;
    }
  })();

  var out = (function (res) {
` + script + `
  })(res);

  if (out === undefined || out === null) return {};
  if (typeof out !== "object") throw new Error("script must return an object");

  var r = {};
  if ("status" in out) {
    var s = Number(out.status);
    if (!isFinite(s)) throw new Error("status must be a number");
    r.status = s;
  }
  if ("headers" in out) r.headers = __strlistmap(out.headers, "headers");
  if ("body" in out) ` + bodyOutJS("r", "out.body") + `
  return r;
})`
}
