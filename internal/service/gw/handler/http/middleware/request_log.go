package middleware

import (
	"log/slog"
	"net/http"
	"time"
)

func NewRequestLog(enabled bool) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			startAt := time.Now()
			rw := &responseStatusWriter{ResponseWriter: w}

			next.ServeHTTP(rw, r)

			statusCode := rw.statusCode
			if statusCode == 0 {
				statusCode = http.StatusOK
			}

			logArgs := []any{
				"status", statusCode,
				"method", r.Method,
				"path", r.URL.Path,
				"query", r.URL.RawQuery,
				// "headers", r.Header,
				"host", r.Host,
				"remote_addr", r.RemoteAddr,
				"user_agent", r.UserAgent(),
				"duration", time.Since(startAt).String(),
			}

			if statusCode < http.StatusOK || statusCode > http.StatusMultipleChoices-1 {
				slog.Info("gw request error", logArgs...)
			} else if enabled {
				slog.Info("gw request", logArgs...)
			}
		})
	}
}

type responseStatusWriter struct {
	http.ResponseWriter
	statusCode int
}

func (w *responseStatusWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}
