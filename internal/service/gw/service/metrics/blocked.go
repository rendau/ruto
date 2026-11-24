package metrics

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/rendau/ruto/internal/infra/metrics"
)

var (
	// No client label on purpose: during an attack every blocked address
	// would become a series of its own.
	blockedTotal = func() *prometheus.CounterVec {
		if !metrics.Enabled {
			return nil
		}
		return metrics.Factory.NewCounterVec(prometheus.CounterOpts{
			Name: "gw_blocked_total",
			Help: "Requests the gateway refused because the client is on the block list.",
		}, []string{metricsLabelProtocol})
	}()

	blocklistRules = func() prometheus.Gauge {
		if !metrics.Enabled {
			return nil
		}
		return metrics.Factory.NewGauge(prometheus.GaugeOpts{
			Name: "gw_blocklist_rules",
			Help: "Rules of the block list the gateway applies, as of the last update.",
		})
	}()
)

// Blocked returns the counter of refused requests of a protocol ("http",
// "grpc"), or nil when metrics are off.
func Blocked(protocol string) prometheus.Counter {
	if blockedTotal == nil {
		return nil
	}
	return blockedTotal.WithLabelValues(protocol)
}

func SetBlocklistRules(n int) {
	if blocklistRules != nil {
		blocklistRules.Set(float64(n))
	}
}
