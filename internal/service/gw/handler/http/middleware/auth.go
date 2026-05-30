package middleware

import (
	"net/http"

	domAppModel "github.com/rendau/ruto/internal/domain/app/model"
	domEndpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
	domRootModel "github.com/rendau/ruto/internal/domain/root/model"
	"github.com/rendau/ruto/internal/service/gw/service/auth"
	authModel "github.com/rendau/ruto/internal/service/gw/service/auth/model"
)

func NewAuth(
	root *domRootModel.Root,
	app *domAppModel.App,
	ep *domEndpointModel.Endpoint,
) Middleware {
	service := auth.New(root, app, ep)
	if service == nil {
		return func(next http.Handler) http.Handler { return next }
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authRequest := authModel.NewAuthRequest()
			authRequest.SetHttpHeader(r.Header)
			authRequest.SetHttpQueryParams(r.URL.Query())
			authRequest.SetRemoteAddr(r.RemoteAddr)
			if service.Check(authRequest) {
				next.ServeHTTP(w, r)
			} else {
				http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			}
		})
	}
}
