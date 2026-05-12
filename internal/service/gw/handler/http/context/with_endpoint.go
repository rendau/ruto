package context

import (
	"context"

	endpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
)

type endpointContextKey struct{}

func WithEndpoint(ctx context.Context, endpoint *endpointModel.Endpoint) context.Context {
	return context.WithValue(ctx, endpointContextKey{}, endpoint)
}

func ExtractEndpoint(ctx context.Context) *endpointModel.Endpoint {
	endpoint, ok := ctx.Value(endpointContextKey{}).(*endpointModel.Endpoint)
	if ok {
		return endpoint
	}
	return nil
}
