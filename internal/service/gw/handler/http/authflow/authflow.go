// Package authflow is the gateway's login session at an OpenID Connect
// provider: the authorization code flow with PKCE for the gateway's own
// client, ending in the access and refresh cookies that the jwt auth method
// reads on every other request, the renewal of those cookies with the refresh
// token, and the logout. It serves no routes of its own: the broker starts a
// login (Login), serves the provider's return address
// /_auth/<provider>/callback with Callback and builds the fronts' tokens on
// top of the session.
//
// A login unit is a provider with its default client, shared by every app that
// names the provider. Units know nothing about proxying, apps' endpoints or
// the cache.
package authflow

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/rendau/ruto/internal/constant"
	authModel "github.com/rendau/ruto/internal/domain/auth/model"
	rootModel "github.com/rendau/ruto/internal/domain/root/model"
)

type Unit struct {
	name     string // provider name, for logs
	provider rootModel.RootJwt
	client   *authModel.JwtClient
	// basePath is where the provider's routes live and where the refresh
	// cookie is scoped to: it never travels with an app request.
	basePath   string
	cookieName string // access cookie on Path=/, the one the jwt method reads
	// appCookies are the cookies of the app tokens got for this provider's
	// shared token (App.Jwt): a logout drops them with the session.
	appCookies []authModel.JwtCookie
	publicUrl  string // gateway address the provider sends the browser back to
	secure     bool   // https gateway: Secure cookies, __Host- login cookies
}

// NewUnits returns a login unit for every login provider of the snapshot, by
// provider name.
func NewUnits(snapshot *rootModel.Root) map[string]*Unit {
	units := map[string]*Unit{}
	for i := range snapshot.Jwt {
		p := &snapshot.Jwt[i]
		if !p.LoginEnabled() || p.Client == nil {
			continue
		}
		units[p.Name] = newUnit(snapshot, p)
	}
	return units
}

func newUnit(snapshot *rootModel.Root, provider *rootModel.RootJwt) *Unit {
	publicUrl := provider.PublicUrl
	if publicUrl == "" {
		publicUrl = snapshot.BaseUrl
	}

	return &Unit{
		name:       provider.Name,
		provider:   *provider,
		client:     provider.Client,
		basePath:   "/" + constant.AuthRoutePrefix + "/" + provider.Name,
		cookieName: constant.JwtCookieName(provider.Name),
		appCookies: appCookies(snapshot, provider.Name),
		publicUrl:  publicUrl,
		secure:     strings.HasPrefix(publicUrl, "https://"),
	}
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

func (u *Unit) redirectUri() string {
	return u.publicUrl + u.basePath + "/callback"
}

func (u *Unit) logWarn(msg string, err error, r *http.Request) {
	slog.Warn("authflow: "+msg, "unit", u.name, "error", err, "remote", r.RemoteAddr)
}
