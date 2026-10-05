package middleware

import (
	"net/http"
	"strings"

	appModel "github.com/rendau/ruto/internal/domain/app/model"
	endpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
	"github.com/rendau/ruto/internal/service/gw/handler/http/rw_wrapper"
	"github.com/rendau/ruto/internal/service/gw/service/seen"
)

// NewSeenPaths records which concrete paths a wildcard endpoint serves. It is a
// no-op for regular endpoints: their path is already known.
func NewSeenPaths(app *appModel.App, ep *endpointModel.Endpoint) Middleware {
	if !ep.Http.HasWildcard() {
		return func(next http.Handler) http.Handler { return next }
	}

	collector := seen.Ins()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rw, ok := w.(*rw_wrapper.Wrapper)
			if !ok {
				rw = rw_wrapper.New(w)
			}
			path := strings.TrimPrefix(r.URL.Path, app.PathPrefix)

			next.ServeHTTP(rw, r)

			collector.Record(app.Id, ep.Id, r.Method, path, rw.GetStatusCode() == http.StatusNotFound)
		})
	}
}
