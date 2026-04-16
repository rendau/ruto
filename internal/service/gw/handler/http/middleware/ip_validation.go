package middleware

import (
	"net"
	"net/http"
)

func NewIPValidation(allowedIPs []string) Middleware {
	allowedSet := make(map[string]struct{}, len(allowedIPs))
	for _, ip := range allowedIPs {
		allowedSet[ip] = struct{}{}
	}

	if len(allowedSet) == 0 {
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestIP := extractRemoteIP(r.RemoteAddr)
			if _, ok := allowedSet[requestIP]; !ok {
				http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func extractRemoteIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return remoteAddr
	}
	return host
}
