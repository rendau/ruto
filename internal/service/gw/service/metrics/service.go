package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"

	domAppModel "github.com/rendau/ruto/internal/domain/app/model"
	domEndpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
	"github.com/rendau/ruto/internal/infra/metrics"
)

const (
	metricsLabelApp    = "app"
	metricsLabelMethod = "method"
	metricsLabelStatus = "status"
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
			metricsLabelApp,
			metricsLabelMethod,
			metricsLabelStatus,
		})
	}()

	httpRequestDurationSeconds = func() *prometheus.HistogramVec {
		if !metrics.Enabled {
			return nil
		}
		return metrics.Factory.NewHistogramVec(prometheus.HistogramOpts{
			Name: "gw_http_request_duration_seconds",
			Help: "Gateway HTTP request duration in seconds.",
			Buckets: []float64{
				0.005,
				0.02,
				0.1,
				0.5,
				2,
			},
		}, []string{
			metricsLabelApp,
			metricsLabelMethod,
		})
	}()
)

type serveFunc func() string

type Service struct {
	app    *domAppModel.App
	ep     *domEndpointModel.Endpoint
	method string
}

func New(
	app *domAppModel.App,
	ep *domEndpointModel.Endpoint,
	method string,
) *Service {
	if httpRequestsTotal == nil || httpRequestDurationSeconds == nil {
		return nil
	}

	return &Service{
		app:    app,
		ep:     ep,
		method: method,
	}
}

func (s *Service) Serve(f serveFunc) {
	startAt := time.Now()

	status := f()

	httpRequestsTotal.WithLabelValues(
		s.app.Name,
		s.method,
		status,
	).Inc()

	httpRequestDurationSeconds.WithLabelValues(
		s.app.Name,
		s.method,
	).Observe(time.Since(startAt).Seconds())
}
