package auth

import (
	"net/http"

	"github.com/samber/lo"

	appModel "github.com/rendau/ruto/internal/domain/app/model"
	authModel "github.com/rendau/ruto/internal/domain/auth/model"
	endpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
	rootModel "github.com/rendau/ruto/internal/domain/root/model"
	"github.com/rendau/ruto/internal/service/gw/handler/http/middleware"
	"github.com/rendau/ruto/internal/service/gw/handler/http/middleware/auth/api_key"
	"github.com/rendau/ruto/internal/service/gw/handler/http/middleware/auth/basic"
	"github.com/rendau/ruto/internal/service/gw/handler/http/middleware/auth/ip_validation"
	"github.com/rendau/ruto/internal/service/gw/handler/http/middleware/auth/jwt"
)

func New(
	root *rootModel.Root,
	app *appModel.App,
	ep *endpointModel.Endpoint,
	jwkGetter jwt.JwkGetterI,
) middleware.Middleware {
	if !ep.Auth.Enabled {
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	methodsAuthorizers := lo.FilterMap(ep.Auth.Methods, func(v authModel.AuthMethod, _ int) ([]authorizerI, bool) {
		result := make([]authorizerI, 0, 4)

		if v.Basic != nil {
			result = append(result, basic.New(v.Basic))
		}
		if v.APIKey != nil {
			result = append(result, api_key.New(v.APIKey))
		}
		if v.JWT != nil {
			result = append(result, jwt.New(jwkGetter, v.JWT))
		}
		if v.IPValidation != nil {
			result = append(result, ip_validation.New(v.IPValidation))
		}

		return result, len(result) > 0
	})

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

func authorizeMethod(authorizers []authorizerI, r *http.Request) bool {
	for _, authorizer := range authorizers {
		if !authorizer.Authorize(r) {
			return false
		}
	}
	return true
}
