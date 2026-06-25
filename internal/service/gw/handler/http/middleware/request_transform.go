package middleware

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	endpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
	"github.com/rendau/ruto/internal/service/gw/handler/http/rw_wrapper"
	"github.com/rendau/ruto/internal/service/gw/service/transform"
)

// NewRequestTransform builds the per-endpoint transform middleware. defaultMaxWorkers
// is the Root-level cap used when the endpoint does not set its own.
func NewRequestTransform(ep *endpointModel.Endpoint, defaultMaxWorkers int) Middleware {
	script := ep.Transform.Request
	if script == "" {
		return func(next http.Handler) http.Handler { return next }
	}

	maxWorkers := ep.Transform.MaxWorkers
	if maxWorkers <= 0 {
		maxWorkers = defaultMaxWorkers
	}

	transformer, err := transform.New(script, maxWorkers)
	if err != nil {
		// A script that does not compile breaks only this endpoint; the rest of
		// the snapshot keeps working. Fail its requests instead of silently
		// proxying them unchanged.
		slog.Error("request transform: compile failed", "error", err, "app_id", ep.AppId, "endpoint_id", ep.Id)
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				failExpected(w, http.StatusInternalServerError)
			})
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, err := io.ReadAll(r.Body)
			_ = r.Body.Close()
			if err != nil {
				failExpected(w, http.StatusBadRequest)
				return
			}

			res, err := transformer.Transform(r.Context(), &transform.Request{
				Method:  r.Method,
				Path:    r.URL.Path,
				Headers: r.Header,
				Params:  r.URL.Query(),
				Body:    body,
				Vars:    ep.Variables,
			})
			if err != nil {
				// Client gone or saturated worker pool: don't treat as a script bug.
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					if r.Context().Err() != nil {
						return
					}
					failExpected(w, http.StatusServiceUnavailable)
					return
				}
				slog.Error("request transform: run failed", "error", err, "app_id", ep.AppId, "endpoint_id", ep.Id)
				failExpected(w, http.StatusBadGateway)
				return
			}

			applyTransform(r, res, body)

			next.ServeHTTP(w, r)
		})
	}
}

func applyTransform(r *http.Request, res *transform.Result, origBody []byte) {
	if res.Method != nil {
		r.Method = *res.Method
	}

	if res.Path != nil {
		p := *res.Path
		if !strings.HasPrefix(p, "/") {
			p = "/" + p
		}
		r.URL.Path = p
		r.URL.RawPath = ""
	}

	if res.Headers != nil {
		h := make(http.Header, len(res.Headers))
		for k, vs := range res.Headers {
			for _, v := range vs {
				h.Add(k, v)
			}
		}
		r.Header = h
	}

	if res.Params != nil {
		q := make(url.Values, len(res.Params))
		for k, vs := range res.Params {
			q[k] = vs
		}
		r.URL.RawQuery = q.Encode()
	}

	// The body was consumed by io.ReadAll, so it must be reset either way:
	// to the script's value when set, otherwise to the original bytes.
	body := origBody
	if res.BodySet {
		body = res.Body
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	r.ContentLength = int64(len(body))
	r.Header.Set("Content-Length", strconv.Itoa(len(body)))
}

// failExpected writes a status without a body and marks the response as a
// gateway-originated non-2xx so request logging does not treat it as a backend error.
func failExpected(w http.ResponseWriter, status int) {
	if rw, ok := w.(*rw_wrapper.Wrapper); ok {
		rw.MarkExpected()
	}
	w.WriteHeader(status)
}
