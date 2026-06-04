package middleware

import (
	"net/http"

	domAppModel "github.com/rendau/ruto/internal/domain/app/model"
	domEndpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
	"github.com/rendau/ruto/internal/service/gw/handler/http/rw_wrapper"
	"github.com/rendau/ruto/internal/service/gw/service/log"
)

func NewRequestLog(
	app *domAppModel.App,
	ep *domEndpointModel.Endpoint,
	routePath string,
	accessLog bool,
) Middleware {
	if ep.Http.Method != "" {
		routePath = ep.Http.Method + " " + routePath
	}

	service := log.New(app, ep, routePath, accessLog)
	if service == nil {
		return func(next http.Handler) http.Handler { return next }
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			service.Serve(func() ([]any, string, error) {
				rw := rw_wrapper.New(w)
				next.ServeHTTP(rw, r)
				return []any{
					// "headers", r.Header,
					"host", r.Host,
					"remote_addr", r.RemoteAddr,
					"path", r.URL.Path,
					"query_params", r.URL.RawQuery,
				}, rw.GetStatusCodeStr(), rw.GetStatusCodeErr()
			})
		})
	}
}
