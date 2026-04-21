package middleware

import (
	"context"
	"net/http"

	"github.com/rendau/ruto/internal/model/config"
)

type endpointContextKey struct{}

func NewWithEndpoint(endpoint *config.Endpoint) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), endpointContextKey{}, endpoint)))
		})
	}
}

func EndpointFromRequest(r *http.Request) *config.Endpoint {
	endpoint, ok := r.Context().Value(endpointContextKey{}).(*config.Endpoint)
	if ok {
		return endpoint
	}
	return nil
}
