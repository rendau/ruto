package prometheus

import (
	"context"
	"time"

	prometheusModel "github.com/rendau/ruto/internal/service/prometheus/model"
)

type Prometheus interface {
	Ping(ctx context.Context) error
	QueryRange(ctx context.Context, query string, start, end time.Time, step time.Duration) ([]prometheusModel.Series, error)
}
