package middleware

import (
	"net/http"
	"strings"

	"github.com/rendau/ruto/internal/model/config"
)

func NewCors(conf config.RootCors) Middleware {
	if !conf.Enabled {
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	allowedOrigins := make(map[string]struct{}, len(conf.AllowOrigins))
	allowAnyOrigin := len(conf.AllowOrigins) == 0
	for _, origin := range conf.AllowOrigins {
		if origin == "*" {
			allowAnyOrigin = true
			continue
		}
		allowedOrigins[origin] = struct{}{}
	}

	allowMethods := strings.Join(conf.AllowMethods, ", ")
	allowHeaders := strings.Join(conf.AllowHeaders, ", ")

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			allowedOrigin, originAllowed := resolveAllowedOrigin(origin, conf.AllowCredentials, allowAnyOrigin, allowedOrigins)
			if originAllowed {
				h := w.Header()
				h.Add("Vary", "Origin")
				h.Set("Access-Control-Allow-Origin", allowedOrigin)
				if conf.AllowCredentials {
					h.Set("Access-Control-Allow-Credentials", "true")
				}
			}

			if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
				if origin != "" && !originAllowed {
					http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
					return
				}

				h := w.Header()
				h.Add("Vary", "Access-Control-Request-Method")
				h.Add("Vary", "Access-Control-Request-Headers")
				if allowMethods != "" {
					h.Set("Access-Control-Allow-Methods", allowMethods)
				}

				requestHeaders := r.Header.Get("Access-Control-Request-Headers")
				switch {
				case allowHeaders != "":
					h.Set("Access-Control-Allow-Headers", allowHeaders)
				case requestHeaders != "":
					h.Set("Access-Control-Allow-Headers", requestHeaders)
				}

				if conf.MaxAge != "" {
					h.Set("Access-Control-Max-Age", conf.MaxAge)
				}

				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func resolveAllowedOrigin(origin string, allowCredentials bool, allowAnyOrigin bool, allowedOrigins map[string]struct{}) (string, bool) {
	if origin == "" {
		return "", false
	}

	if allowAnyOrigin {
		if allowCredentials {
			return origin, true
		}
		return "*", true
	}

	if _, ok := allowedOrigins[origin]; ok {
		return origin, true
	}

	return "", false
}
