package middleware

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	endpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
	"github.com/rendau/ruto/internal/service/gw/service/transform"
)

// NewResponseTransform builds the per-endpoint response transform middleware. It
// buffers the backend response, runs the script, then writes the (possibly
// reshaped) status/headers/body to the client. Buffering means streaming
// responses are fully read before the client sees them — only endpoints with a
// response script pay this cost.
func NewResponseTransform(ep *endpointModel.Endpoint, defaultMaxWorkers int) Middleware {
	script := ep.Transform.Response
	if script == "" {
		return func(next http.Handler) http.Handler { return next }
	}

	maxWorkers := ep.Transform.MaxWorkers
	if maxWorkers <= 0 {
		maxWorkers = defaultMaxWorkers
	}

	transformer, err := transform.NewResponse(script, maxWorkers)
	if err != nil {
		slog.Error("response transform: compile failed", "error", err, "app_id", ep.AppId, "endpoint_id", ep.Id)
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				failExpected(w, http.StatusInternalServerError)
			})
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capt := &responseCapture{header: make(http.Header), status: http.StatusOK}
			next.ServeHTTP(capt, r)

			res, err := transformer.Transform(r.Context(), &transform.Response{
				Status:  capt.status,
				Headers: capt.header,
				Body:    capt.body.Bytes(),
				Vars:    ep.Variables,
			})
			if err != nil {
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					if r.Context().Err() != nil {
						return
					}
				} else {
					slog.Error("response transform: run failed", "error", err, "app_id", ep.AppId, "endpoint_id", ep.Id)
				}
				// The backend response is already consumed and cannot be replayed
				// reliably, so surface a gateway error rather than a partial body.
				failExpected(w, http.StatusBadGateway)
				return
			}

			writeTransformedResponse(w, capt, res)
		})
	}
}

func writeTransformedResponse(w http.ResponseWriter, cap *responseCapture, res *transform.ResponseResult) {
	dst := w.Header()

	if res.Headers != nil {
		for k, vs := range res.Headers {
			for i, v := range vs {
				if i == 0 {
					dst.Set(k, v)
				} else {
					dst.Add(k, v)
				}
			}
		}
	} else {
		for k, vs := range cap.header {
			dst[k] = vs
		}
	}

	body := cap.body.Bytes()
	if res.BodySet {
		body = res.Body
	}
	// We send a complete, buffered body — fix the framing headers accordingly.
	dst.Set("Content-Length", strconv.Itoa(len(body)))
	dst.Del("Transfer-Encoding")

	status := cap.status
	if res.Status != nil {
		status = *res.Status
	}

	w.WriteHeader(status)
	_, _ = w.Write(body)
}

// responseCapture buffers status, headers and body written by the proxy so the
// response script can reshape them before anything reaches the client.
type responseCapture struct {
	header      http.Header
	status      int
	body        bytes.Buffer
	wroteHeader bool
}

func (c *responseCapture) Header() http.Header { return c.header }

func (c *responseCapture) WriteHeader(status int) {
	if c.wroteHeader {
		return
	}
	c.status = status
	c.wroteHeader = true
}

func (c *responseCapture) Write(p []byte) (int, error) {
	if !c.wroteHeader {
		c.WriteHeader(http.StatusOK)
	}
	return c.body.Write(p)
}
