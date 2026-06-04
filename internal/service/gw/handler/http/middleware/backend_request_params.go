package middleware

import (
	"net/http"

	endpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
)

func NewBackendRequestParams(ep *endpointModel.Endpoint) Middleware {
	if len(ep.Backend.Headers) == 0 && len(ep.Backend.QueryParams) == 0 {
		return func(next http.Handler) http.Handler { return next }
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if len(ep.Backend.QueryParams) > 0 {
				query := r.URL.Query()
				for key, value := range ep.Backend.QueryParams {
					query.Set(key, value)
				}
				r.URL.RawQuery = query.Encode()
			}

			if len(ep.Backend.Headers) > 0 {
				for key, value := range ep.Backend.Headers {
					r.Header.Set(key, value)
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}
