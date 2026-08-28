package prometheus

import (
	"context"
	"time"

	prometheusModel "github.com/rendau/ruto/internal/service/prometheus/model"
)

type Prometheus interface {
	QueryRange(ctx context.Context, query string, start, end time.Time, step time.Duration) ([]prometheusModel.Series, error)
}
