package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/rendau/ruto/internal/infra/metrics"
	"github.com/rendau/ruto/internal/service/gw/handler/http/request"
)

const (
	metricsLabelAppName     = "app_name"
	metricsLabelHTTPMethod  = "http_method"
	metricsLabelFullPath    = "full_path"
	metricsLabelAuthEnabled = "auth_enabled"
	metricsLabelStatusCode  = "status_code"

	metricsLabelUnknown = "__unknown__"
)

var (
	httpRequestsTotal = func() *prometheus.CounterVec {
		if !metrics.Enabled {
			return nil
		}
		return metrics.Factory.NewCounterVec(prometheus.CounterOpts{
			Name: "gw_http_requests_total",
			Help: "Total number of gateway HTTP requests.",
		}, []string{
			metricsLabelAppName,
			metricsLabelHTTPMethod,
			metricsLabelFullPath,
			metricsLabelStatusCode,
			metricsLabelAuthEnabled,
		})
	}()

	httpRequestDurationSeconds = func() *prometheus.HistogramVec {
		if !metrics.Enabled {
			return nil
		}
		return metrics.Factory.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "gw_http_request_duration_seconds",
			Help:    "Gateway HTTP request duration in seconds.",
			Buckets: prometheus.DefBuckets,
		}, []string{
			metricsLabelAppName,
			metricsLabelHTTPMethod,
			metricsLabelFullPath,
			metricsLabelStatusCode,
		})
	}()
)

func NewMetrics() Middleware {
	if !metrics.Enabled || httpRequestsTotal == nil || httpRequestDurationSeconds == nil {
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			startAt := time.Now()

			appName := metricsLabelUnknown
			fullPath := metricsLabelUnknown
			authEnabled := false

			ctxReq := request.Extract(r.Context())
			if ctxReq != nil {
				appName = ctxReq.App.Name
				fullPath = resolveFullPath(ctxReq)
				authEnabled = ctxReq.Endpoint.Auth.Enabled
			}

			rw := &responseStatusWriter{ResponseWriter: w}
			next.ServeHTTP(rw, r)

			statusCode := rw.statusCode
			if statusCode == 0 {
				statusCode = http.StatusOK
			}
			statusCodeStr := strconv.Itoa(statusCode)

			httpRequestsTotal.WithLabelValues(
				appName,
				r.Method,
				fullPath,
				statusCodeStr,
				strconv.FormatBool(authEnabled),
			).Inc()
			httpRequestDurationSeconds.WithLabelValues(
				appName,
				r.Method,
				fullPath,
				statusCodeStr,
			).Observe(time.Since(startAt).Seconds())
		})
	}
}

func resolveFullPath(ctxReq *request.Request) string {
	if ctxReq.Endpoint.Path == "" {
		return ctxReq.App.PathPrefix
	}

	return ctxReq.App.PathPrefix + "/" + ctxReq.Endpoint.Path
}
