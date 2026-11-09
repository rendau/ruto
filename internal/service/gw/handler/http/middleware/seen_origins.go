package middleware

import (
	"net/http"

	"github.com/rendau/ruto/internal/service/gw/service/seen"
)

// NewSeenOrigins records the Origin header of every request, whether or not
// CORS admits it: the point is to show the admin which fronts call the gateway
// so they can be added to allow_origins.
func NewSeenOrigins() Middleware {
	collector := seen.OriginsIns()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if origin := r.Header.Get("Origin"); origin != "" {
				collector.Record(origin)
			}
			next.ServeHTTP(w, r)
		})
	}
}
