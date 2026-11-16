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
// What is served so far: the discovery document and ruto's public keys. The
// provider's other pages (account, registration) are sent to the provider; the
// broker's own endpoints (authorize, token, userinfo, logout) answer 404 until
// they are built.
package broker

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/rendau/ruto/internal/constant"
	rootModel "github.com/rendau/ruto/internal/domain/root/model"
	authModel "github.com/rendau/ruto/internal/service/gw/service/auth/model"
	"github.com/rendau/ruto/internal/service/gw/service/oidc"
)

// reservedClients are taken by the gateway's own routes under
// /_auth/<provider>.
var reservedClients = map[string]struct{}{
	"login": {}, "callback": {}, "refresh": {}, "logout": {}, "me": {}, "logged-out": {},
}

// clientPattern keeps a client name a single plain path segment.
var clientPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._~-]{0,127}$`)

// MetadataGetter reads the provider's discovery document.
type MetadataGetter interface {
	Metadata(ctx context.Context, issuer, internalUrl string) (*oidc.Metadata, error)
}

// KeySet gives ruto's public keys.
type KeySet interface {
	JWKS() ([]byte, bool)
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
	meta         MetadataGetter
	keys         KeySet
}

// Register adds the broker routes of every login provider to router. The
// gateway's own routes of a provider are registered as plain paths, which chi
// prefers to the client parameter.
func Register(router chi.Router, snapshot *rootModel.Root, meta MetadataGetter, keys KeySet) {
	for i := range snapshot.Jwt {
		p := &snapshot.Jwt[i]
		if !p.LoginEnabled() || p.Client == nil {
			continue
		}
		u, err := newUnit(snapshot, p, meta, keys)
		if err != nil {
			slog.Error("broker: provider skipped", "provider", p.Name, "error", err)
			continue
		}
		router.Handle(u.basePath+"/{client}", http.HandlerFunc(u.serve))
		router.Handle(u.basePath+"/{client}/*", http.HandlerFunc(u.serve))
	}
}

func newUnit(snapshot *rootModel.Root, p *rootModel.RootJwt, meta MetadataGetter, keys KeySet) (*unit, error) {
	issuer, err := url.Parse(p.Issuer)
	if err != nil {
		return nil, err
	}
	publicUrl := p.PublicUrl
	if publicUrl == "" {
		publicUrl = snapshot.BaseUrl
	}
	return &unit{
		name:         p.Name,
		issuer:       p.Issuer,
		internalUrl:  p.InternalUrl,
		issuerOrigin: issuer.Scheme + "://" + issuer.Host,
		issuerPath:   strings.TrimRight(issuer.EscapedPath(), "/"),
		basePath:     "/" + constant.AuthRoutePrefix + "/" + p.Name,
		publicUrl:    publicUrl,
		meta:         meta,
		keys:         keys,
	}, nil
}

func (u *unit) serve(w http.ResponseWriter, r *http.Request) {
	client := chi.URLParam(r, "client")
	if _, reserved := reservedClients[client]; reserved || !clientPattern.MatchString(client) {
		http.NotFound(w, r)
		return
	}
	front := u.basePath + "/" + client
	rest, ok := strings.CutPrefix(r.URL.EscapedPath(), front)
	if !ok {
		http.NotFound(w, r)
		return
	}

	meta, err := u.meta.Metadata(r.Context(), u.issuer, u.internalUrl)
	if err != nil {
		slog.Warn("broker: provider discovery failed", "provider", u.name, "error", err)
		http.Error(w, "provider unavailable", http.StatusBadGateway)
		return
	}

	if rest == u.issuerPath+"/.well-known/openid-configuration" {
		if !readMethod(r) {
			methodNotAllowed(w)
			return
		}
		u.discovery(w, meta, client)
		return
	}
	if p := endpointPath(meta.PublicEndpoint("jwks_uri")); p != "" && rest == p {
		if !readMethod(r) {
			methodNotAllowed(w)
			return
		}
		u.jwks(w)
		return
	}

	if ownEndpoint(meta, rest) {
		http.NotFound(w, r)
		return
	}

	// The provider's own pages (account, registration) the SDK links to: a
	// browser goes there. A call with a ruto token is a call to ruto.
	if readMethod(r) && !authModel.IsRutoToken(bearer(r)) {
		target := u.issuerOrigin + rest
		if r.URL.RawQuery != "" {
			target += "?" + r.URL.RawQuery
		}
		http.Redirect(w, r, target, http.StatusFound)
		return
	}

	http.NotFound(w, r)
}

// ownEndpoint tells the paths of the endpoints the broker answers itself, so
// that they are never sent to the provider.
func ownEndpoint(meta *oidc.Metadata, rest string) bool {
	for _, name := range brokerEndpoints {
		if p := endpointPath(meta.PublicEndpoint(name)); p != "" && p == rest {
			return true
		}
	}
	return false
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

func readMethod(r *http.Request) bool {
	return r.Method == http.MethodGet || r.Method == http.MethodHead
}

func methodNotAllowed(w http.ResponseWriter) {
	w.Header().Set("Allow", "GET, HEAD")
	http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
}

func bearer(r *http.Request) string {
	scheme, token, ok := strings.Cut(r.Header.Get("Authorization"), " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return ""
	}
	return strings.TrimSpace(token)
}
