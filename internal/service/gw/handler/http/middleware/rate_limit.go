package middleware

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/rendau/ruto/internal/service/gw/handler/http/rw_wrapper"
	"github.com/rendau/ruto/internal/service/gw/service/clientstat"
	"github.com/rendau/ruto/internal/service/gw/service/ratelimit"
)

// NewRateLimit rejects a request over a rate-limit rule with 429. It goes
// before auth, so attempts with wrong credentials are counted too: by ip, by
// basic-auth user, by bearer token — a fresh random token is a fresh counter,
// which is why a token rule needs an ip rule beside it.
func NewRateLimit(limiter *ratelimit.Endpoint) Middleware {
	if limiter == nil {
		return func(next http.Handler) http.Handler { return next }
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ok, retryAfter := limiter.Check(ratelimit.Request{
				RemoteAddr:    r.RemoteAddr,
				Header:        r.Header,
				Authorization: strings.TrimSpace(r.Header.Get("Authorization")),
				Client:        clientstat.FromContext(r.Context()),
			})
			if ok {
				next.ServeHTTP(w, r)
				return
			}

			if rw, isWrapper := w.(*rw_wrapper.Wrapper); isWrapper {
				rw.MarkExpected()
			}
			w.Header().Set("Retry-After", strconv.Itoa(ratelimit.RetryAfterSeconds(retryAfter)))
			http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
		})
	}
}
