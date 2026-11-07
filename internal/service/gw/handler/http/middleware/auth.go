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

// SessionRenewerI renews a login session inside an app request (the login
// units of a provider with transparent refresh, authflow.Sessions).
type SessionRenewerI interface {
	// RefreshCookie names the refresh cookie behind accessCookie when its
	// session may be renewed inside a request; "" otherwise.
	RefreshCookie(accessCookie string) string
	// Renew trades the refresh cookie value for a new session, sets the new
	// cookies on w and returns the new access token.
	Renew(w http.ResponseWriter, r *http.Request, accessCookie, refreshCookie string) (string, bool)
}

// renewableCookie is an access cookie of the endpoint's jwt methods whose
// session the gateway renews itself, with the refresh cookie behind it.
type renewableCookie struct {
	access  string
	refresh string
}

func NewAuth(
	ep *domEndpointModel.Endpoint,
	sessions SessionRenewerI,
) Middleware {
	service := auth.New(ep)
	if service == nil {
		return func(next http.Handler) http.Handler { return next }
	}

	renewable := renewableCookies(ep, sessions)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authRequest := newAuthRequest(r)
			result := service.Check(authRequest)

			// An expired session is renewed once and the request checked
			// again with the new token; a denial by role or ip is final.
			if result != authModel.Allowed && !result.Forbidden() && len(renewable) > 0 {
				if renewSession(w, r, authRequest, renewable, sessions) {
					authRequest = newAuthRequest(r)
					result = service.Check(authRequest)
				}
			}

			if result == authModel.Allowed {
				// A token accepted from a cookie also goes to the backend as a
				// bearer, so backends on headers and backends on cookies both
				// work. An Authorization the client sent itself is kept.
				if token := authRequest.CookieToken(); token != "" && r.Header.Get("Authorization") == "" {
					r.Header.Set("Authorization", "Bearer "+token)
				}
				next.ServeHTTP(w, r)
				return
			}

			if rw, ok := w.(*rw_wrapper.Wrapper); ok {
				rw.MarkExpected()
			}
			code := http.StatusUnauthorized
			if result.Forbidden() {
				code = http.StatusForbidden
			}
			http.Error(w, http.StatusText(code), code)
		})
	}
}

func newAuthRequest(r *http.Request) *authModel.AuthRequest {
	authRequest := authModel.NewAuthRequest()
	authRequest.SetHttpHeader(r.Header)
	authRequest.SetHttpQueryParams(r.URL.Query())
	authRequest.SetRemoteAddr(r.RemoteAddr)
	return authRequest
}

func renewableCookies(ep *domEndpointModel.Endpoint, sessions SessionRenewerI) []renewableCookie {
	if sessions == nil {
		return nil
	}
	var result []renewableCookie
	for _, method := range ep.Auth.Methods {
		if method == nil || method.JWT == nil || method.JWT.CookieName == "" {
			continue
		}
		refresh := sessions.RefreshCookie(method.JWT.CookieName)
		if refresh == "" {
			continue
		}
		item := renewableCookie{access: method.JWT.CookieName, refresh: refresh}
		if !slices.Contains(result, item) {
			result = append(result, item)
		}
	}
	return result
}

// renewSession renews the first session whose access token was missing or
// expired and whose refresh cookie came along. The new access token replaces
// the stale one in the request, so the check and the backend both see it.
func renewSession(
	w http.ResponseWriter,
	r *http.Request,
	authRequest *authModel.AuthRequest,
	renewable []renewableCookie,
	sessions SessionRenewerI,
) bool {
	for _, item := range renewable {
		if !authRequest.CookieStale(item.access) {
			continue
		}
		refreshValue := authRequest.ExtractCookie(item.refresh)
		if refreshValue == "" {
			continue
		}
		token, ok := sessions.Renew(w, r, item.access, refreshValue)
		if !ok {
			continue
		}
		setRequestCookie(r.Header, item.access, token)
		return true
	}
	return false
}

// setRequestCookie puts name=value into the Cookie header of a request in
// place of every value the name had, or adds it. The header ends up as one
// line, the way RFC 6265 has a client send it.
func setRequestCookie(header http.Header, name, value string) {
	lines := header.Values("Cookie")
	pairs := make([]string, 0, 8)
	replaced := false
	for _, line := range lines {
		for pair := range strings.SplitSeq(line, ";") {
			pair = strings.TrimSpace(pair)
			if pair == "" {
				continue
			}
			if pairName, _, _ := strings.Cut(pair, "="); pairName == name {
				if replaced {
					continue
				}
				pair = name + "=" + value
				replaced = true
			}
			pairs = append(pairs, pair)
		}
	}
	if !replaced {
		pairs = append(pairs, name+"="+value)
	}
	header.Set("Cookie", strings.Join(pairs, "; "))
}
