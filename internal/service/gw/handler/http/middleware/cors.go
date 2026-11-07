package middleware

import (
	"net/http"
	"strings"

	"github.com/samber/lo"

	rootModel "github.com/rendau/ruto/internal/domain/root/model"
)

func NewCors(conf rootModel.RootCors) Middleware {
	if !conf.Enabled {
		return func(next http.Handler) http.Handler { return next }
	}

	origins := rootModel.NewOriginMatcher(conf.AllowOrigins)

	// With credentials a browser takes "*" literally, so "*" means: echo what
	// the preflight asks for.
	anyMethod := lo.Contains(conf.AllowMethods, rootModel.AnyOrigin)
	anyHeader := lo.Contains(conf.AllowHeaders, rootModel.AnyOrigin)
	allowMethods := strings.Join(conf.AllowMethods, ", ")
	allowHeaders := strings.Join(conf.AllowHeaders, ", ")
	maxAge := conf.MaxAgeSeconds()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			allowedOrigin, originIsAllowed := resolveAllowedOrigin(origin, conf.AllowCredentials, origins)
			if originIsAllowed {
				h := w.Header()
				h.Add("Vary", "Origin")
				h.Set("Access-Control-Allow-Origin", allowedOrigin)
				if conf.AllowCredentials {
					h.Set("Access-Control-Allow-Credentials", "true")
				}
			}

			if r.Method == http.MethodOptions {
				if origin != "" && !originIsAllowed {
					http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
					return
				}

				h := w.Header()
				h.Add("Vary", "Access-Control-Request-Method")
				h.Add("Vary", "Access-Control-Request-Headers")
				requestMethod := r.Header.Get("Access-Control-Request-Method")
				switch {
				case anyMethod && requestMethod != "":
					h.Set("Access-Control-Allow-Methods", requestMethod)
				case allowMethods != "":
					h.Set("Access-Control-Allow-Methods", allowMethods)
				}

				requestHeaders := r.Header.Get("Access-Control-Request-Headers")
				switch {
				case requestHeaders != "" && (anyHeader || allowHeaders == ""):
					h.Set("Access-Control-Allow-Headers", requestHeaders)
				case allowHeaders != "":
					h.Set("Access-Control-Allow-Headers", allowHeaders)
				}

				if maxAge != "" {
					h.Set("Access-Control-Max-Age", maxAge)
				}

				w.WriteHeader(http.StatusNoContent)

				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// resolveAllowedOrigin returns the Access-Control-Allow-Origin value for the
// request origin. "*" is echoed as the request origin when credentials are
// allowed, since a browser rejects a literal "*" with cookies.
func resolveAllowedOrigin(origin string, allowCredentials bool, origins *rootModel.OriginMatcher) (string, bool) {
	if origin == "" {
		return "", false
	}
	if origins.Any() {
		if allowCredentials {
			return origin, true
		}
		return rootModel.AnyOrigin, true
	}
	if origins.Listed(origin) {
		return origin, true
	}
	return "", false
}
