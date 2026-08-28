package monitoring

import (
	"context"
	"time"

	endpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
	logstoreModel "github.com/rendau/ruto/internal/service/logstore/model"
	prometheusModel "github.com/rendau/ruto/internal/service/prometheus/model"
)

type PrometheusServiceI interface {
	QueryRange(ctx context.Context, query string, start, end time.Time, step time.Duration) ([]prometheusModel.Series, error)
}

type LogStoreServiceI interface {
	List(ctx context.Context, pars *logstoreModel.ListReq) ([]*logstoreModel.Entry, error)
}

type EndpointServiceI interface {
	Get(ctx context.Context, id string, errNE bool) (*endpointModel.Endpoint, bool, error)
}

type SessionServiceI interface {
	CtxIsAuthorized(ctx context.Context) bool
	CtxHasFullAppAccess(ctx context.Context) bool
	CtxGetAppIds(ctx context.Context) []string
}
