package middleware

import (
	"net/http"

	"github.com/rendau/ruto/internal/service/gw/service/blocklist"
	"github.com/rendau/ruto/internal/service/gw/service/clientstat"
)

// NewBlocklist refuses a blocked client with 403. It wraps the whole handler,
// CORS and the router included: a blocked browser gets 403 on a preflight
// too, and an unknown route costs the client no more than a known one.
func NewBlocklist(guard *blocklist.Guard) Middleware {
	if guard == nil {
		return func(next http.Handler) http.Handler { return next }
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if guard.BlockedClient(clientstat.FromContext(r.Context()), r.RemoteAddr, r.Header) {
				http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
