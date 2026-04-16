package middleware

import (
	"net/http"
	"time"
)

func NewTimeout(timeout time.Duration) Middleware {
	if timeout <= 0 {
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	return func(next http.Handler) http.Handler {
		return http.TimeoutHandler(next, timeout, http.StatusText(http.StatusGatewayTimeout))
	}
}
