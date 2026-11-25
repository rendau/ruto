package middleware

import (
	"net/http"

	"github.com/rendau/ruto/internal/service/gw/service/clientstat"
)

// NewClientStat counts every request for its client and hands the resolved
// client down in the context, for the block list and rate limits to read its
// address and mark what they refuse. It is the outermost layer: blocked
// requests, preflights and unknown routes are counted too.
func NewClientStat(meter *clientstat.Meter) Middleware {
	if meter == nil {
		return func(next http.Handler) http.Handler { return next }
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			client := meter.Start(r.RemoteAddr, r.Header)
			next.ServeHTTP(w, r.WithContext(clientstat.WithClient(r.Context(), client)))
		})
	}
}
