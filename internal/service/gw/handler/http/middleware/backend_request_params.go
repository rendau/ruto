package middleware

import (
	"net/http"

	appModel "github.com/rendau/ruto/internal/domain/app/model"
)

func NewBackendRequestParams(params appModel.BackendRequestParams) Middleware {
	if len(params.Headers) == 0 && len(params.QueryParams) == 0 {
		return func(next http.Handler) http.Handler { return next }
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if len(params.QueryParams) > 0 {
				query := r.URL.Query()
				for key, value := range params.QueryParams {
					query.Set(key, value)
				}
				r.URL.RawQuery = query.Encode()
			}

			if len(params.Headers) > 0 {
				for key, value := range params.Headers {
					r.Header.Set(key, value)
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}
