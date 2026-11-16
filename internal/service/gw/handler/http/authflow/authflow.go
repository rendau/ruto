// Package authflow serves the gateway's own login routes,
// /_auth/<provider>/login|callback|refresh|logout|me: the OpenID Connect
// authorization code flow with PKCE, ending in the access and refresh cookies
// that the jwt auth method reads on every other request, the renewal of those
// cookies with the refresh token, and a front's view of the session (who is
// logged in, with what roles).
//
// A login unit is a provider with its default client, shared by every app that
// names the provider. Units know nothing about proxying, apps' endpoints or
// the cache.
package authflow

import (
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/rendau/ruto/internal/constant"
	authModel "github.com/rendau/ruto/internal/domain/auth/model"
	rootModel "github.com/rendau/ruto/internal/domain/root/model"
)

type Unit struct {
	name     string // provider name, for logs
	provider rootModel.RootJwt
	client   *authModel.JwtClient
	// basePath is where the unit's routes live and where the refresh cookie is
	// scoped to: it never travels with an app request.
	basePath   string
	cookieName string // access cookie on Path=/, the one the jwt method reads
	// appCookies are the cookies of the app tokens got for this provider's
	// shared token (App.Jwt): a logout drops them with the session.
	appCookies []authModel.JwtCookie
	publicUrl  string // gateway address the provider sends the browser back to
	secure     bool   // https gateway: Secure cookies, __Host- login cookie
	// A login may send the browser back to the gateway itself or to a CORS
	// origin (the fronts); "*" does not count, it would be an open redirect.
	publicOrigin string
	origins      *rootModel.OriginMatcher
}

// Register adds the routes of every login unit of the snapshot to router and
// returns the units by provider name: the broker logs fronts in through them.
func Register(router chi.Router, snapshot *rootModel.Root) map[string]*Unit {
	units := map[string]*Unit{}
	origins := rootModel.NewOriginMatcher(snapshot.Cors.AllowOrigins)
	for i := range snapshot.Jwt {
		p := &snapshot.Jwt[i]
		if !p.LoginEnabled() || p.Client == nil {
			continue
		}
		u := newUnit(snapshot, p, origins)
		u.register(router)
		units[p.Name] = u
	}
	return units
}

func newUnit(snapshot *rootModel.Root, provider *rootModel.RootJwt, origins *rootModel.OriginMatcher) *Unit {
	publicUrl := provider.PublicUrl
	if publicUrl == "" {
		publicUrl = snapshot.BaseUrl
	}

	u := &Unit{
		name:         provider.Name,
		provider:     *provider,
		client:       provider.Client,
		basePath:     "/" + constant.AuthRoutePrefix + "/" + provider.Name,
		cookieName:   constant.JwtCookieName(provider.Name),
		appCookies:   appCookies(snapshot, provider.Name),
		publicUrl:    publicUrl,
		secure:       strings.HasPrefix(publicUrl, "https://"),
		publicOrigin: originOf(publicUrl),
		origins:      origins,
	}
	return u
}

func appCookies(snapshot *rootModel.Root, provider string) []authModel.JwtCookie {
	var result []authModel.JwtCookie
	for _, app := range snapshot.ActiveApps() {
		if app.JwtAudience(provider) != "" {
			result = append(result, authModel.JwtCookie{
				Name: constant.JwtAppCookieName(provider, app.PathPrefix),
				Path: app.PathPrefix,
			})
		}
	}
	return result
}

func (u *Unit) register(router chi.Router) {
	router.Get(u.basePath+"/login", u.login)
	router.Get(u.basePath+"/callback", u.callback)
	// POST only: a SameSite=Lax cookie does not travel with a cross-site POST,
	// so another site cannot make the browser renew the session.
	router.Post(u.basePath+"/refresh", u.refresh)
	router.Get(u.basePath+"/logout", u.logout)
	router.Get(u.basePath+"/me", u.me)
}

// originOf returns "scheme://host[:port]" of an absolute http(s) url, lower
// case; "" for anything else.
func originOf(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return ""
	}
	return strings.ToLower(parsed.Scheme + "://" + parsed.Host)
}

// returnUrl validates the address a login or logout sends the browser to: a
// path on the gateway, or an absolute url on an allowed origin. Empty means
// the gateway root.
func (u *Unit) returnUrl(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return u.publicUrl + "/", true
	}
	if strings.HasPrefix(raw, "/") {
		// "//host" and "/\host" are protocol-relative to a browser.
		if strings.HasPrefix(raw, "//") || strings.HasPrefix(raw, "/\\") {
			return "", false
		}
		return u.publicUrl + raw, true
	}
	origin := originOf(raw)
	if origin == "" {
		return "", false
	}
	if origin != u.publicOrigin && !u.origins.Listed(origin) {
		return "", false
	}
	return raw, true
}

func (u *Unit) redirectUri() string {
	return u.publicUrl + u.basePath + "/callback"
}

func (u *Unit) logWarn(msg string, err error, r *http.Request) {
	slog.Warn("authflow: "+msg, "unit", u.name, "error", err, "remote", r.RemoteAddr)
}
