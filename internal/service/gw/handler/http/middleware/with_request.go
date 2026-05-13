package middleware

import (
	"net/http"

	appModel "github.com/rendau/ruto/internal/domain/app/model"
	endpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
	rootModel "github.com/rendau/ruto/internal/domain/root/model"
	requestModel "github.com/rendau/ruto/internal/service/gw/handler/http/request"
	"github.com/rendau/ruto/internal/service/gw/jwk"
)

func NewWithRequest(
	root *rootModel.Root,
	app *appModel.App,
	ep *endpointModel.Endpoint,
	jwkService *jwk.Service,
) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(requestModel.Inject(r.Context(), &requestModel.Request{
				Root:       root,
				App:        app,
				Endpoint:   ep,
				JwkService: jwkService,
			})))
		})
	}
}
