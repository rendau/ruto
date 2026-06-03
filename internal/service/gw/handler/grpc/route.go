package grpc

import (
	"context"

	gogrpc "google.golang.org/grpc"

	"github.com/rendau/ruto/internal/domain/app/model"
	domEndpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
)

type route struct {
	app               *model.App
	endpoint          *domEndpointModel.Endpoint
	targetGrpcAddress string
	backendHeaders    map[string]string
	handler           gogrpc.StreamHandler
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
