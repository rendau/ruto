package middleware

import (
	"net/http"

	domAppModel "github.com/rendau/ruto/internal/domain/app/model"
	domEndpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
	"github.com/rendau/ruto/internal/service/gw/handler/http/rw_wrapper"
	"github.com/rendau/ruto/internal/service/gw/service/metrics"
)

func NewMetrics(
	app *domAppModel.App,
	ep *domEndpointModel.Endpoint,
	routePath string,
) Middleware {
	if ep.Method != "" {
		routePath = ep.Method + " " + routePath
	}

	service := metrics.New(app, ep, routePath)
	if service == nil {
		return func(next http.Handler) http.Handler { return next }
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			service.Serve(func() string {
				rw := rw_wrapper.New(w)
				next.ServeHTTP(rw, r)
				return rw.GetStatusCodeStr()
			})
		})
	}
}
