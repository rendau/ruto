package auth

import (
	"net/http"

	"github.com/samber/lo"

	endpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
	"github.com/rendau/ruto/internal/service/gw/handler/http/middleware"
	"github.com/rendau/ruto/internal/service/gw/handler/http/middleware/auth/api_key"
	"github.com/rendau/ruto/internal/service/gw/handler/http/middleware/auth/basic"
	"github.com/rendau/ruto/internal/service/gw/handler/http/middleware/auth/ip_validation"
	"github.com/rendau/ruto/internal/service/gw/handler/http/middleware/auth/jwt"
)

func New(endpoint *endpointModel.Endpoint) middleware.Middleware {
	if !endpoint.Auth.Enabled {
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	methodsAuthorizers := buildAuthorizers(endpoint)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for _, methodAuthorizers := range methodsAuthorizers {
				if authorizeMethod(methodAuthorizers, r) {
					next.ServeHTTP(w, r)
					return
				}
			}

			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		})
	}
}

func buildAuthorizers(endpoint *endpointModel.Endpoint) [][]authorizerI {
	return lo.FilterMap(endpoint.Auth.Methods, func(v endpointModel.AuthMethod, _ int) ([]authorizerI, bool) {
		result := make([]authorizerI, 0, 4)

		if v.Basic != nil {
			result = append(result, basic.New(v.Basic))
		}
		if v.APIKey != nil {
			result = append(result, api_key.New(v.APIKey))
		}
		if v.JWT != nil {
			result = append(result, jwt.New(v.JWT))
		}
		if v.IPValidation != nil {
			result = append(result, ip_validation.New(v.IPValidation))
		}

		return result, len(result) > 0
	})
}

func authorizeMethod(authorizers []authorizerI, r *http.Request) bool {
	for _, authorizer := range authorizers {
		if !authorizer.Authorize(r) {
			return false
		}
	}
	return true
}
