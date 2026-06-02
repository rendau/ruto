package grpc

import (
	"context"

	"github.com/rendau/ruto/internal/domain/app/model"
	domEndpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
	serviceAuth "github.com/rendau/ruto/internal/service/gw/service/auth"
	serviceLog "github.com/rendau/ruto/internal/service/gw/service/log"
	serviceMetrics "github.com/rendau/ruto/internal/service/gw/service/metrics"
)

type route struct {
	app               *model.App
	endpoint          *domEndpointModel.Endpoint
	targetGrpcAddress string
	auth              *serviceAuth.Service
	log               *serviceLog.Service
	metrics           *serviceMetrics.Service
}

type routeCtxKeyT struct{}

var routeCtxKey = routeCtxKeyT{}

func contextWithRoute(ctx context.Context, rt *route) context.Context {
	return context.WithValue(ctx, routeCtxKey, rt)
}

func routeFromContext(ctx context.Context) (*route, bool) {
	rt, ok := ctx.Value(routeCtxKey).(*route)
	return rt, ok
}
