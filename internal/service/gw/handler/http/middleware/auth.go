package middleware

import (
	"net/http"

	domEndpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
	"github.com/rendau/ruto/internal/service/gw/handler/http/rw_wrapper"
	"github.com/rendau/ruto/internal/service/gw/service/auth"
	authModel "github.com/rendau/ruto/internal/service/gw/service/auth/model"
)

func NewAuth(
	ep *domEndpointModel.Endpoint,
) Middleware {
	service := auth.New(ep)
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
				if rw, ok := w.(*rw_wrapper.Wrapper); ok {
					rw.MarkExpected()
				}
				http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			}
		})
	}
}
