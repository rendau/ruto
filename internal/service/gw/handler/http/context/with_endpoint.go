package context

import (
	"context"

	"github.com/rendau/ruto/internal/model/config"
)

type endpointContextKey struct{}

func WithEndpoint(ctx context.Context, endpoint *config.Endpoint) context.Context {
	return context.WithValue(ctx, endpointContextKey{}, endpoint)
}

func ExtractEndpoint(ctx context.Context) *config.Endpoint {
	endpoint, ok := ctx.Value(endpointContextKey{}).(*config.Endpoint)
	if ok {
		return endpoint
	}
	return nil
}
