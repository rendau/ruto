package broker

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appModel "github.com/rendau/ruto/internal/domain/app/model"
	authModel "github.com/rendau/ruto/internal/domain/auth/model"
	rootModel "github.com/rendau/ruto/internal/domain/root/model"
	snapshotModel "github.com/rendau/ruto/internal/domain/snapshot/model"
	gwAuthModel "github.com/rendau/ruto/internal/service/gw/service/auth/model"
	"github.com/rendau/ruto/internal/service/gw/service/oidc"
	"github.com/rendau/ruto/internal/service/gw/service/rutokey"
)

const keycloakDoc = `{
  "issuer": "http://kc.test/realms/corp",
  "authorization_endpoint": "http://kc.test/realms/corp/protocol/openid-connect/auth",
  "token_endpoint": "http://kc.test/realms/corp/protocol/openid-connect/token",
  "userinfo_endpoint": "http://kc.test/realms/corp/protocol/openid-connect/userinfo",
  "end_session_endpoint": "http://kc.test/realms/corp/protocol/openid-connect/logout",
  "jwks_uri": "http://kc.test/realms/corp/protocol/openid-connect/certs",
  "introspection_endpoint": "http://kc.test/realms/corp/protocol/openid-connect/token/introspect",
  "registration_endpoint": "http://kc.test/realms/corp/clients-registrations/openid-connect",
  "device_authorization_endpoint": "http://kc.test/realms/corp/protocol/openid-connect/auth/device",
  "grant_types_supported": ["authorization_code", "implicit", "refresh_token", "password", "client_credentials",
    "urn:ietf:params:oauth:grant-type:token-exchange"],
  "response_types_supported": ["code", "none", "id_token", "token", "code id_token"],
  "response_modes_supported": ["query", "fragment", "form_post", "query.jwt"],
  "code_challenge_methods_supported": ["plain", "S256"],
  "token_endpoint_auth_methods_supported": ["private_key_jwt", "client_secret_basic", "client_secret_post"],
  "id_token_signing_alg_values_supported": ["RS256", "ES256"],
  "request_parameter_supported": true,
  "scopes_supported": ["openid", "profile", "email", "roles"],
  "claims_supported": ["sub", "name", "email"],
  "subject_types_supported": ["public", "pairwise"]
}`

// authDoc is a provider at the root of its host, like auth in mechta.
const authDoc = `{
  "issuer": "https://id.test",
  "authorization_endpoint": "https://id.test/oauth/authorize",
  "token_endpoint": "https://id.test/oauth/token",
  "userinfo_endpoint": "https://id.test/oauth/userinfo",
  "jwks_uri": "https://id.test/.well-known/jwks.json"
}`

type fakeMeta struct {
	docs map[string]string // by issuer
	err  error
}

func (f *fakeMeta) Metadata(_ context.Context, issuer, _ string) (*oidc.Metadata, error) {
	if f.err != nil {
		return nil, f.err
	}
	meta := new(oidc.Metadata)
	if err := json.Unmarshal([]byte(f.docs[issuer]), &meta.Document); err != nil {
		return nil, err
	}
	return meta, nil
}

func newRouter(t *testing.T, meta MetadataGetter, keys KeySet) http.Handler {
	t.Helper()
	return newRouterWith(t, Deps{Meta: meta, Keys: keys}, newFakeSessions(t))
}

// newRouterWith builds the broker for the test snapshot: providers corp
// (Keycloak-like) and id (at its host root), apps with audiences crm (front
// and backend share the client) and orders.
func newRouterWith(t *testing.T, deps Deps, sessions *fakeSessions) http.Handler {
	t.Helper()
	snapshot := &rootModel.Root{
		BaseUrl: "https://api.test",
		Cors:    rootModel.RootCors{AllowOrigins: []string{"https://*.front.test", "*"}},
		Jwt: []rootModel.RootJwt{
			{Name: "corp", Issuer: "http://kc.test/realms/corp", Client: &authModel.JwtClient{Id: "ruto"}},
			{Name: "id", Issuer: "https://id.test", PublicUrl: "https://gw.test", Client: &authModel.JwtClient{Id: "ruto"}},
			{Name: "nologin", Issuer: "https://other.test"},
		},
		Apps: []*appModel.App{
			{Name: "crm-api", PathPrefix: "/crm-api", Active: true, Jwt: []appModel.AppJwt{{Provider: "corp", Audience: "crm"}}},
			{Name: "orders", PathPrefix: "/orders", Active: true, Jwt: []appModel.AppJwt{{Provider: "corp", Audience: "orders"}}},
		},
	}
	if deps.Sessions == nil {
		deps.Sessions = map[string]Sessions{"corp": sessions, "id": sessions}
	}
	if deps.Exchanger == nil {
		deps.Exchanger = sessions
	}
	if deps.Used == nil {
		deps.Used = newFakeUsed()
	}
	router := chi.NewRouter()
	// the gateway's own routes of a provider, as authflow registers them
	router.Get("/_auth/corp/login", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("own login")) })
	Register(router, snapshot, deps)
	return router
}

func newKeys(t *testing.T) *rutokey.Service {
	t.Helper()
	keys := rutokey.New()
	keys.Set(snapshotModel.SigningKeyFrom("broker test"))
	return keys
}

func get(t *testing.T, h http.Handler, target string, header ...string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	for i := 0; i+1 < len(header); i += 2 {
		req.Header.Set(header[i], header[i+1])
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func discovery(t *testing.T, h http.Handler, target string) map[string]any {
	t.Helper()
	rec := get(t, h, target)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	assert.Equal(t, cacheControl, rec.Header().Get("Cache-Control"))
	doc := map[string]any{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &doc))
	return doc
}

func strs(v any) []string {
	var result []string
	for _, item := range v.([]any) {
		result = append(result, item.(string))
	}
	return result
}

func TestDiscoveryPerFront(t *testing.T) {
	h := newRouter(t, &fakeMeta{docs: map[string]string{"http://kc.test/realms/corp": keycloakDoc}}, newKeys(t))

	crm := discovery(t, h, "/_auth/corp/crm/realms/corp/.well-known/openid-configuration")
	cart := discovery(t, h, "/_auth/corp/cart-fe/realms/corp/.well-known/openid-configuration")

	front := "https://api.test/_auth/corp/crm/realms/corp"
	assert.Equal(t, front, crm["issuer"])
	assert.Equal(t, "https://api.test/_auth/corp/cart-fe/realms/corp", cart["issuer"])
	assert.Equal(t, front+"/protocol/openid-connect/auth", crm["authorization_endpoint"])
	assert.Equal(t, front+"/protocol/openid-connect/token", crm["token_endpoint"])
	assert.Equal(t, front+"/protocol/openid-connect/userinfo", crm["userinfo_endpoint"])
	assert.Equal(t, front+"/protocol/openid-connect/logout", crm["end_session_endpoint"])
	assert.Equal(t, front+"/protocol/openid-connect/certs", crm["jwks_uri"])

	assert.Equal(t, []string{"authorization_code", "refresh_token"}, strs(crm["grant_types_supported"]))
	assert.Equal(t, []string{"code"}, strs(crm["response_types_supported"]))
	assert.Equal(t, []string{"query", "fragment"}, strs(crm["response_modes_supported"]))
	assert.Equal(t, []string{"S256"}, strs(crm["code_challenge_methods_supported"]))
	assert.Equal(t, []string{"none"}, strs(crm["token_endpoint_auth_methods_supported"]))
	assert.Equal(t, []string{"ES256"}, strs(crm["id_token_signing_alg_values_supported"]))
	assert.Equal(t, false, crm["request_uri_parameter_supported"])

	assert.Equal(t, []string{"openid", "profile", "email", "roles"}, strs(crm["scopes_supported"]), "copied as is")
	assert.Equal(t, []string{"public", "pairwise"}, strs(crm["subject_types_supported"]))
	assert.Equal(t, []string{"sub", "name", "email"}, strs(crm["claims_supported"]))

	for _, dropped := range []string{"introspection_endpoint", "registration_endpoint", "device_authorization_endpoint", "request_parameter_supported"} {
		assert.NotContains(t, crm, dropped, "the broker does not do it")
	}
}

func TestDiscoveryProviderAtHostRoot(t *testing.T) {
	h := newRouter(t, &fakeMeta{docs: map[string]string{"https://id.test": authDoc}}, newKeys(t))

	doc := discovery(t, h, "/_auth/id/cart-fe/.well-known/openid-configuration")
	assert.Equal(t, "https://gw.test/_auth/id/cart-fe", doc["issuer"], "the provider's public_url wins over base_url")
	assert.Equal(t, "https://gw.test/_auth/id/cart-fe/oauth/authorize", doc["authorization_endpoint"])
	assert.Equal(t, "https://gw.test/_auth/id/cart-fe/.well-known/jwks.json", doc["jwks_uri"])
	assert.NotContains(t, doc, "end_session_endpoint", "the provider has none")
	assert.Equal(t, []string{"public"}, strs(doc["subject_types_supported"]), "required, filled in")
}

func TestJWKS(t *testing.T) {
	keys := newKeys(t)
	h := newRouter(t, &fakeMeta{docs: map[string]string{"http://kc.test/realms/corp": keycloakDoc}}, keys)

	rec := get(t, h, "/_auth/corp/crm/realms/corp/protocol/openid-connect/certs")
	require.Equal(t, http.StatusOK, rec.Code)
	want, _ := keys.JWKS()
	assert.JSONEq(t, string(want), rec.Body.String(), "ruto's keys only, not the provider's")

	var set struct {
		Keys []struct {
			Kid string `json:"kid"`
		} `json:"keys"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &set))
	require.Len(t, set.Keys, 1)
	signed, err := keys.Sign(jwt.MapClaims{"sub": "u-1"})
	require.NoError(t, err)
	parsed, _, err := jwt.NewParser().ParseUnverified(signed, jwt.MapClaims{})
	require.NoError(t, err)
	assert.Equal(t, set.Keys[0].Kid, parsed.Header["kid"])

	noKey := newRouter(t, &fakeMeta{docs: map[string]string{"http://kc.test/realms/corp": keycloakDoc}}, rutokey.New())
	assert.Equal(t, http.StatusServiceUnavailable,
		get(t, noKey, "/_auth/corp/crm/realms/corp/protocol/openid-connect/certs").Code)
}

func TestProviderPagesRedirect(t *testing.T) {
	h := newRouter(t, &fakeMeta{docs: map[string]string{
		"http://kc.test/realms/corp": keycloakDoc,
		"https://id.test":            authDoc,
	}}, newKeys(t))

	rec := get(t, h, "/_auth/id/cart-fe/account/profile?tab=security")
	assert.Equal(t, http.StatusFound, rec.Code)
	assert.Equal(t, "https://id.test/account/profile?tab=security", rec.Header().Get("Location"))

	rec = get(t, h, "/_auth/corp/crm/realms/corp/account")
	assert.Equal(t, http.StatusFound, rec.Code)
	assert.Equal(t, "http://kc.test/realms/corp/account", rec.Header().Get("Location"))

	rec = get(t, h, "/_auth/corp/crm/realms/corp/account", "Authorization", "Bearer some-provider-token")
	assert.Equal(t, http.StatusFound, rec.Code, "only a ruto token makes it a call to ruto")

	req := httptest.NewRequest(http.MethodPost, "/_auth/corp/crm/realms/corp/account", strings.NewReader("x=1"))
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNotFound, rec.Code, "only a browser visit goes to the provider")
}

func TestProviderPagesWithRutoToken(t *testing.T) {
	keys := newKeys(t)
	gwAuthModel.SetRutoKids(keys.Kids())
	t.Cleanup(func() { gwAuthModel.SetRutoKids(nil) })
	h := newRouter(t, &fakeMeta{docs: map[string]string{"http://kc.test/realms/corp": keycloakDoc}}, keys)

	token, err := keys.Sign(jwt.MapClaims{"sub": "u-1"})
	require.NoError(t, err)
	rec := get(t, h, "/_auth/corp/crm/realms/corp/account", "Authorization", "Bearer "+token)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestOwnEndpointsAreNotSentToTheProvider(t *testing.T) {
	h := newRouter(t, &fakeMeta{docs: map[string]string{"http://kc.test/realms/corp": keycloakDoc}}, newKeys(t))

	for p, status := range map[string]int{
		"auth":     http.StatusBadRequest, // served: no redirect_uri
		"token":    http.StatusMethodNotAllowed,
		"userinfo": http.StatusNotFound, // not built yet
		"logout":   http.StatusNotFound,
	} {
		rec := get(t, h, "/_auth/corp/crm/realms/corp/protocol/openid-connect/"+p+"?client_id=crm")
		assert.Equal(t, status, rec.Code, p)
		assert.Empty(t, rec.Header().Get("Location"), p)
	}
}

func TestClientNames(t *testing.T) {
	h := newRouter(t, &fakeMeta{docs: map[string]string{"http://kc.test/realms/corp": keycloakDoc}}, newKeys(t))

	rec := get(t, h, "/_auth/corp/login")
	assert.Equal(t, "own login", rec.Body.String(), "the gateway's own route wins")

	for _, client := range []string{"login", "callback", "refresh", "logout", "me", "logged-out", ".hidden", "a%2Fb"} {
		rec := get(t, h, "/_auth/corp/"+client+"/realms/corp/.well-known/openid-configuration")
		assert.Equal(t, http.StatusNotFound, rec.Code, client)
	}

	assert.Equal(t, http.StatusNotFound,
		get(t, h, "/_auth/nologin/crm/.well-known/openid-configuration").Code, "a provider without login has no broker")
}

func TestProviderUnavailable(t *testing.T) {
	h := newRouter(t, &fakeMeta{err: errors.New("down")}, newKeys(t))
	assert.Equal(t, http.StatusBadGateway,
		get(t, h, "/_auth/corp/crm/realms/corp/.well-known/openid-configuration").Code)
	assert.Equal(t, http.StatusBadGateway, get(t, h, "/_auth/corp/crm/realms/corp/account").Code,
		"without the document the broker cannot tell its own endpoints, so nothing goes to the provider")
}
