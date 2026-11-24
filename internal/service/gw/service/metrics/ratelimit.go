package metrics

import (
	"github.com/prometheus/client_golang/prometheus"

	domAppModel "github.com/rendau/ruto/internal/domain/app/model"
	domEndpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
	"github.com/rendau/ruto/internal/infra/metrics"
)

const (
	metricsLabelScope = "scope"
	metricsLabelRule  = "rule"
)

var (
	rateLimitRejectedTotal = func() *prometheus.CounterVec {
		if !metrics.Enabled {
			return nil
		}
		return metrics.Factory.NewCounterVec(prometheus.CounterOpts{
			Name: "gw_rate_limit_rejected_total",
			Help: "Requests the gateway rejected by a rate-limit rule.",
		}, []string{
			metricsLabelApp,
			metricsLabelAppId,
			metricsLabelEndpointId,
			metricsLabelProtocol,
			metricsLabelScope,
			metricsLabelRule,
		})
	}()

	rateLimitEvictedTotal = func() prometheus.Counter {
		if !metrics.Enabled {
			return nil
		}
		return metrics.Factory.NewCounter(prometheus.CounterOpts{
			Name: "gw_rate_limit_evicted_total",
			Help: "Rate-limit counters dropped before expiry because the key table was full (RATE_LIMIT_MAX_KEYS).",
		})
	}()

	rateLimitKeys = func() prometheus.Gauge {
		if !metrics.Enabled {
			return nil
		}
		return metrics.Factory.NewGauge(prometheus.GaugeOpts{
			Name: "gw_rate_limit_keys",
			Help: "Rate-limit counters held in memory, as of the last sweep.",
		})
	}()
)

// RateLimitRejected returns the rejection counter of a rule on an endpoint, or
// nil when metrics are off or the app/endpoint is excluded from them.
func RateLimitRejected(app *domAppModel.App, ep *domEndpointModel.Endpoint, scope, rule string) prometheus.Counter {
	if rateLimitRejectedTotal == nil || app.ExcludeFromMetrics || ep.ExcludeFromMetrics {
		return nil
	}
	return rateLimitRejectedTotal.WithLabelValues(
		app.Name,
		app.Id,
		ep.Id,
		protocolFromEndpoint(ep),
		scope,
		rule,
	)
}

func IncRateLimitEvicted() {
	if rateLimitEvictedTotal != nil {
		rateLimitEvictedTotal.Inc()
	}
}

func SetRateLimitKeys(n int) {
	if rateLimitKeys != nil {
		rateLimitKeys.Set(float64(n))
	}
}
