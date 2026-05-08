package context

import (
	"context"

	"github.com/rendau/ruto/internal/domain/config/model"
)

type endpointContextKey struct{}

func WithEndpoint(ctx context.Context, endpoint *model.Endpoint) context.Context {
	return context.WithValue(ctx, endpointContextKey{}, endpoint)
}

func ExtractEndpoint(ctx context.Context) *model.Endpoint {
	endpoint, ok := ctx.Value(endpointContextKey{}).(*model.Endpoint)
	if ok {
		return endpoint
	}
	return nil
}
