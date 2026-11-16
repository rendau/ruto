package middleware

import (
	"net/http"
	"slices"
	"strings"

	domEndpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
	"github.com/rendau/ruto/internal/service/gw/handler/http/rw_wrapper"
	"github.com/rendau/ruto/internal/service/gw/service/auth"
	authModel "github.com/rendau/ruto/internal/service/gw/service/auth/model"
)

func NewAuth(
	ep *domEndpointModel.Endpoint,
) Middleware {
	service := auth.New(ep)
	if service == nil {
		return func(next http.Handler) http.Handler { return next }
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authRequest := authModel.NewAuthRequest()
			authRequest.SetContext(r.Context())
			authRequest.SetHttpHeader(r.Header)
			authRequest.SetHttpQueryParams(r.URL.Query())
			authRequest.SetRemoteAddr(r.RemoteAddr)
			result := service.Check(authRequest)
			for _, cookie := range authRequest.ResponseCookies() {
				http.SetCookie(w, cookie)
			}
			if result == authModel.Allowed {
				// A token accepted from a cookie also goes to the backend as a
				// bearer, so backends on headers and backends on cookies both
				// work. An Authorization the client sent itself is kept, unless
				// it is a token of the gateway itself.
				if token := authRequest.CookieToken(); token != "" &&
					(r.Header.Get("Authorization") == "" || authRequest.RutoBearer()) {
					r.Header.Set("Authorization", "Bearer "+token)
				}
				removeCookies(r.Header, authRequest.StrippedCookies())
				next.ServeHTTP(w, r)
				return
			}

			if rw, ok := w.(*rw_wrapper.Wrapper); ok {
				rw.MarkExpected()
			}
			code := http.StatusUnauthorized
			switch {
			case result == authModel.Unavailable:
				code = http.StatusBadGateway
			case result.Forbidden():
				code = http.StatusForbidden
			case authRequest.Challenge() != "":
				w.Header().Set("WWW-Authenticate", authRequest.Challenge())
			}
			http.Error(w, http.StatusText(code), code)
		})
	}
}

// removeCookies drops the named cookies from the Cookie header of a request.
func removeCookies(header http.Header, names []string) {
	if len(names) == 0 {
		return
	}
	lines := header.Values("Cookie")
	kept := make([]string, 0, 8)
	for _, line := range lines {
		for pair := range strings.SplitSeq(line, ";") {
			pair = strings.TrimSpace(pair)
			name, _, _ := strings.Cut(pair, "=")
			if pair == "" || slices.Contains(names, name) {
				continue
			}
			kept = append(kept, pair)
		}
	}
	if len(kept) == 0 {
		header.Del("Cookie")
		return
	}
	header.Set("Cookie", strings.Join(kept, "; "))
}
