// Package authflow serves the gateway's own login routes,
// /_auth/<provider>[/<app prefix>]/login|callback|logout: the OpenID Connect
// authorization code flow with PKCE, ending in the access and refresh cookies
// that the jwt auth method reads on every other request.
//
// A login unit is a provider plus a client: the provider's default client
// (shared by every app that names the provider) or an app's own client (its own
// cookie, scoped to the app's path). Units know nothing about proxying, apps'
// endpoints or the cache.
package authflow

import (
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/rendau/ruto/internal/constant"
	appModel "github.com/rendau/ruto/internal/domain/app/model"
	authModel "github.com/rendau/ruto/internal/domain/auth/model"
	rootModel "github.com/rendau/ruto/internal/domain/root/model"
)

type unit struct {
	name     string // "<provider>" or "<provider>/<app prefix>", for logs
	provider rootModel.RootJwt
	client   *authModel.JwtClient
	// basePath is where the unit's routes live and where the refresh cookie is
	// scoped to: it never travels with an app request.
	basePath   string
	cookieName string // access cookie, the one the jwt method reads
	cookiePath string // "/" for the shared login, the app's prefix for its own
	publicUrl  string // gateway address the provider sends the browser back to
	secure     bool   // https gateway: Secure cookies, __Host- login cookie
	// A login may send the browser back to the gateway itself or to a CORS
	// origin (the fronts); "*" does not count, it would be an open redirect.
	publicOrigin string
	origins      *rootModel.OriginMatcher
}

// Register adds the routes of every login unit of the snapshot to router.
func Register(router chi.Router, snapshot *rootModel.Root) {
	providers := make(map[string]*rootModel.RootJwt, len(snapshot.Jwt))
	for i := range snapshot.Jwt {
		p := &snapshot.Jwt[i]
		if p.LoginEnabled() {
			providers[p.Name] = p
		}
	}
	if len(providers) == 0 {
		return
	}

	origins := rootModel.NewOriginMatcher(snapshot.Cors.AllowOrigins)

	for _, p := range providers {
		if p.Client == nil {
			continue
		}
		newUnit(snapshot, p, p.Client, nil, origins).register(router)
	}

	for _, app := range snapshot.ActiveApps() {
		for i := range app.Jwt {
			p, ok := providers[app.Jwt[i].Provider]
			if !ok {
				slog.Warn("authflow: app refers to an unknown login provider, its login is skipped",
					"app", app.Name, "provider", app.Jwt[i].Provider)
				continue
			}
			newUnit(snapshot, p, &app.Jwt[i].Client, app, origins).register(router)
		}
	}
}

func newUnit(
	snapshot *rootModel.Root,
	provider *rootModel.RootJwt,
	client *authModel.JwtClient,
	app *appModel.App,
	origins *rootModel.OriginMatcher,
) *unit {
	publicUrl := provider.PublicUrl
	if publicUrl == "" {
		publicUrl = snapshot.BaseUrl
	}

	u := &unit{
		name:         provider.Name,
		provider:     *provider,
		client:       client,
		basePath:     "/" + constant.AuthRoutePrefix + "/" + provider.Name,
		cookieName:   constant.JwtCookieName(provider.Name),
		cookiePath:   "/",
		publicUrl:    publicUrl,
		secure:       strings.HasPrefix(publicUrl, "https://"),
		publicOrigin: originOf(publicUrl),
		origins:      origins,
	}
	if app != nil {
		u.name += app.PathPrefix
		u.basePath += app.PathPrefix
		u.cookieName = constant.JwtAppCookieName(provider.Name, app.PathPrefix)
		u.cookiePath = app.PathPrefix
	}

	return u
}

func (u *unit) register(router chi.Router) {
	router.Get(u.basePath+"/login", u.login)
	router.Get(u.basePath+"/callback", u.callback)
	router.Get(u.basePath+"/logout", u.logout)
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
func (u *unit) returnUrl(raw string) (string, bool) {
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

func (u *unit) redirectUri() string {
	return u.publicUrl + u.basePath + "/callback"
}

func (u *unit) logWarn(msg string, err error, r *http.Request) {
	slog.Warn("authflow: "+msg, "unit", u.name, "error", err, "remote", r.RemoteAddr)
}
