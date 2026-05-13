package auth

import (
	"net/http"

	endpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
	"github.com/rendau/ruto/internal/service/gw/handler/http/middleware"
	"github.com/rendau/ruto/internal/service/gw/handler/http/middleware/auth/jwt"
)

type authorizerI interface {
	Authorize(r *http.Request) bool
}

func New(endpoint *endpointModel.Endpoint) middleware.Middleware {
	if !endpoint.Auth.Enabled {
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	authorizers := make([]authorizerI, 0, len(endpoint.Auth.Methods))
	for _, method := range endpoint.Auth.Methods {
		switch {
		case method.Basic != nil:
			authorizers = append(authorizers, newBasicAuthorizer(method.Basic))
		case method.APIKey != nil:
			authorizers = append(authorizers, newAPIKeyAuthorizer(method.APIKey))
		case method.JWT != nil:
			authorizers = append(authorizers, jwt.New(method.JWT))
		}
	}

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
