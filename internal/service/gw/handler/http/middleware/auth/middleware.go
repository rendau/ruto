package auth

import (
	"net/http"

	"github.com/samber/lo"

	endpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
	"github.com/rendau/ruto/internal/service/gw/handler/http/middleware"
	"github.com/rendau/ruto/internal/service/gw/handler/http/middleware/auth/api_key"
	"github.com/rendau/ruto/internal/service/gw/handler/http/middleware/auth/basic"
	"github.com/rendau/ruto/internal/service/gw/handler/http/middleware/auth/jwt"
)

func New(endpoint *endpointModel.Endpoint) middleware.Middleware {
	if !endpoint.Auth.Enabled {
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	authorizers := buildAuthorizers(endpoint)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for _, method := range authorizers {
				if method.Authorize(r) {
					next.ServeHTTP(w, r)
					return
				}
			}

			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		})
	}
}

func buildAuthorizers(endpoint *endpointModel.Endpoint) []authorizerI {
	return lo.FilterMap(endpoint.Auth.Methods, func(v endpointModel.AuthMethod, _ int) (authorizerI, bool) {
		switch {
		case v.Basic != nil:
			return basic.New(v.Basic), true
		case v.APIKey != nil:
			return api_key.New(v.APIKey), true
		case v.JWT != nil:
			return jwt.New(v.JWT), true
		}
		return nil, false
	})

}
