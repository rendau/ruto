// Package broker serves the gateway as an OpenID Connect provider for the
// fronts, standing before the real provider of the snapshot: a front logs in
// "to ruto" at /_auth/<provider>/<client>/… with its own standard SDK, and the
// gateway talks to the provider on behalf of its own client.
//
// Every front has its own address: the provider's issuer path under
// /_auth/<provider>/<client>, so the broker knows the front from the url and
// the cookies of one front never travel to the routes of another. There is no
// list of fronts: any client name is served, the return addresses are checked
// by the CORS origins.
//
// What is served: the discovery document, ruto's public keys, the
// authorization endpoint and the code exchange at the token endpoint. The
// provider's other pages (account, registration) are sent to the provider;
// userinfo, the token refresh and logout are not built yet (404 and
// unsupported_grant_type).
package broker

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"

	"github.com/rendau/ruto/internal/constant"
	appModel "github.com/rendau/ruto/internal/domain/app/model"
	authModel "github.com/rendau/ruto/internal/domain/auth/model"
	rootModel "github.com/rendau/ruto/internal/domain/root/model"
	"github.com/rendau/ruto/internal/service/gw/handler/http/authflow"
	gwAuthModel "github.com/rendau/ruto/internal/service/gw/service/auth/model"
	"github.com/rendau/ruto/internal/service/gw/service/oidc"
)

// MetadataGetter reads the provider's discovery document.
type MetadataGetter interface {
	Metadata(ctx context.Context, issuer, internalUrl string) (*oidc.Metadata, error)
}

// KeySet gives ruto's public keys and signs ruto's tokens.
type KeySet interface {
	JWKS() ([]byte, bool)
	Sign(claims jwt.Claims) (string, error)
}

// Sessions is the provider's login on the gateway (authflow): the shared
// session in the cookies and the login at the provider.
type Sessions interface {
	Session(w http.ResponseWriter, r *http.Request) (*authflow.Session, error)
	LoginPath(returnPath, prompt string) string
	ParseProviderToken(token string) (jwt.MapClaims, error)
	SetCookie(w http.ResponseWriter, name, value, path string, maxAge int)
}

// Exchanger trades the shared token for a token with another audience.
type Exchanger interface {
	Exchange(ctx context.Context, provider, subjectToken, audience string) (string, error)
}

// UsedMarker marks one-time values used across the gateway pods.
type UsedMarker interface {
	MarkUsed(ctx context.Context, key string, ttl time.Duration) bool
}

// Deps are the services the broker works with.
type Deps struct {
	Meta      MetadataGetter
	Keys      KeySet
	Sessions  map[string]Sessions // by provider name
	Exchanger Exchanger
	Used      UsedMarker
}

type unit struct {
	name        string // provider name
	issuer      string // the provider's public address, as configured
	internalUrl string
	// issuerOrigin and issuerPath split the issuer: the provider's pages are
	// sent to issuerOrigin, the broker lives under the provider's paths.
	issuerOrigin string
	issuerPath   string
	basePath     string // /_auth/<provider>
	publicUrl    string // gateway address the fronts see
	publicOrigin string
	origins      *rootModel.OriginMatcher
	// audienceCookies are the cookies of the app tokens by audience: a front
	// token is the app token of the apps whose audience is the front's client.
	audienceCookies map[string][]authModel.JwtCookie
	deps            Deps
	sessions        Sessions
}

// Register adds the broker routes of every login provider to router. The
// gateway's own routes of a provider are registered as plain paths, which chi
// prefers to the client parameter.
func Register(router chi.Router, snapshot *rootModel.Root, deps Deps) {
	origins := rootModel.NewOriginMatcher(snapshot.Cors.AllowOrigins)
	appCookies := appModel.JwtAudienceCookies(snapshot.ActiveApps())
	for i := range snapshot.Jwt {
		p := &snapshot.Jwt[i]
		if !p.LoginEnabled() || p.Client == nil {
			continue
		}
		sessions := deps.Sessions[p.Name]
		if sessions == nil {
			continue
		}
		u, err := newUnit(snapshot, p, origins, appCookies, deps, sessions)
		if err != nil {
			slog.Error("broker: provider skipped", "provider", p.Name, "error", err)
			continue
		}
		router.Handle(u.basePath+"/{client}", http.HandlerFunc(u.serve))
		router.Handle(u.basePath+"/{client}/*", http.HandlerFunc(u.serve))
	}
}

func newUnit(
	snapshot *rootModel.Root,
	p *rootModel.RootJwt,
	origins *rootModel.OriginMatcher,
	appCookies map[string][]authModel.JwtCookie,
	deps Deps,
	sessions Sessions,
) (*unit, error) {
	issuer, err := url.Parse(p.Issuer)
	if err != nil {
		return nil, err
	}
	publicUrl := p.PublicUrl
	if publicUrl == "" {
		publicUrl = snapshot.BaseUrl
	}
	audienceCookies := map[string][]authModel.JwtCookie{}
	prefix := appModel.JwtAudienceKey(p.Name, "")
	for key, cookies := range appCookies {
		if audience, ok := strings.CutPrefix(key, prefix); ok {
			audienceCookies[audience] = cookies
		}
	}
	return &unit{
		name:            p.Name,
		issuer:          p.Issuer,
		internalUrl:     p.InternalUrl,
		issuerOrigin:    issuer.Scheme + "://" + issuer.Host,
		issuerPath:      strings.TrimRight(issuer.EscapedPath(), "/"),
		basePath:        "/" + constant.AuthRoutePrefix + "/" + p.Name,
		publicUrl:       publicUrl,
		publicOrigin:    originOf(publicUrl),
		origins:         origins,
		audienceCookies: audienceCookies,
		deps:            deps,
		sessions:        sessions,
	}, nil
}

func (u *unit) serve(w http.ResponseWriter, r *http.Request) {
	client := chi.URLParam(r, "client")
	if !constant.BrokerClientIsValid(client) {
		http.NotFound(w, r)
		return
	}
	front := u.basePath + "/" + client
	rest, ok := strings.CutPrefix(r.URL.EscapedPath(), front)
	if !ok {
		http.NotFound(w, r)
		return
	}

	meta, err := u.deps.Meta.Metadata(r.Context(), u.issuer, u.internalUrl)
	if err != nil {
		slog.Warn("broker: provider discovery failed", "provider", u.name, "error", err)
		http.Error(w, "provider unavailable", http.StatusBadGateway)
		return
	}

	if rest == u.issuerPath+"/.well-known/openid-configuration" {
		if !readMethod(r) {
			methodNotAllowed(w, "GET, HEAD")
			return
		}
		u.discovery(w, meta, client)
		return
	}

	switch endpoint := endpointOf(meta, rest); endpoint {
	case "jwks_uri":
		if !readMethod(r) {
			methodNotAllowed(w, "GET, HEAD")
			return
		}
		u.jwks(w)
		return
	case "authorization_endpoint":
		u.authorize(w, r, client)
		return
	case "token_endpoint":
		u.token(w, r, client)
		return
	case "":
	default: // not built yet; never sent to the provider
		http.NotFound(w, r)
		return
	}

	// The provider's own pages (account, registration) the SDK links to: a
	// browser goes there. A call with a ruto token is a call to ruto.
	if readMethod(r) && !gwAuthModel.IsRutoToken(bearer(r)) {
		target := u.issuerOrigin + rest
		if r.URL.RawQuery != "" {
			target += "?" + r.URL.RawQuery
		}
		http.Redirect(w, r, target, http.StatusFound)
		return
	}

	http.NotFound(w, r)
}

// frontIssuer is the broker's issuer for the front with client.
func (u *unit) frontIssuer(client string) string {
	return u.publicUrl + u.basePath + "/" + client + u.issuerPath
}

// endpointOf names the member of the provider's document whose endpoint the
// broker answers at rest; "" for none of them.
func endpointOf(meta *oidc.Metadata, rest string) string {
	for _, name := range append([]string{"jwks_uri"}, brokerEndpoints...) {
		if p := endpointPath(meta.PublicEndpoint(name)); p != "" && p == rest {
			return name
		}
	}
	return ""
}

// endpointPath is the escaped path of an endpoint url; "" for none.
func endpointPath(endpoint string) string {
	if endpoint == "" {
		return ""
	}
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Host == "" {
		return ""
	}
	return parsed.EscapedPath()
}

// originOf returns "scheme://host[:port]" of an absolute http(s) url, lower
// case; "" for anything else.
func originOf(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return ""
	}
	return strings.ToLower(parsed.Scheme + "://" + parsed.Host)
}

func readMethod(r *http.Request) bool {
	return r.Method == http.MethodGet || r.Method == http.MethodHead
}

func methodNotAllowed(w http.ResponseWriter, allow string) {
	w.Header().Set("Allow", allow)
	http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
}

func bearer(r *http.Request) string {
	scheme, token, ok := strings.Cut(r.Header.Get("Authorization"), " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return ""
	}
	return strings.TrimSpace(token)
}
