package middleware

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	jwtv5 "github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	authModel "github.com/rendau/ruto/internal/domain/auth/model"
	domEndpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
	rootModel "github.com/rendau/ruto/internal/domain/root/model"
	"github.com/rendau/ruto/internal/service/gw/service/apptoken"
	gwAuthModel "github.com/rendau/ruto/internal/service/gw/service/auth/model"
	"github.com/rendau/ruto/internal/service/gw/service/jwk"
)

// appTokenProvider is a provider that exchanges shared tokens for app tokens:
// the app token carries the requested audience, and "alice" gets the manager
// role of that client. "bob" is refused, "gone" has a dead session, "down"
// makes the provider fail; anyone else gets a token without roles.
type appTokenProvider struct {
	t         *testing.T
	key       *rsa.PrivateKey
	kid       string
	issuer    string
	exchanges atomic.Int32
	delay     time.Duration
}

func newAppTokenProvider(t *testing.T) *appTokenProvider {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	p := &appTokenProvider{t: t, key: key, kid: "corp-kid"}

	mux := http.NewServeMux()
	mux.HandleFunc("/realms/corp/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer":                 p.issuer,
			"authorization_endpoint": p.issuer + "/auth",
			"token_endpoint":         p.issuer + "/token",
			"jwks_uri":               p.issuer + "/certs",
		})
	})
	mux.HandleFunc("/realms/corp/certs", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(jwk.ItemSet{Keys: []*jwk.Item{{
			Kty: "RSA", Use: "sig", Alg: "RS256", Kid: p.kid,
			N: base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
			E: base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()),
		}}})
	})
	mux.HandleFunc("/realms/corp/token", func(w http.ResponseWriter, r *http.Request) {
		p.exchanges.Add(1)
		time.Sleep(p.delay)
		if err := r.ParseForm(); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		subject, _, err := jwtv5.NewParser().ParseUnverified(r.PostForm.Get("subject_token"), jwtv5.MapClaims{})
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		sub, _ := subject.Claims.(jwtv5.MapClaims)["sub"].(string)
		audience := r.PostForm.Get("audience")
		switch sub {
		case "down":
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		case "bob":
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid_request", "error_description": "Requested audience not available: " + audience})
			return
		case "gone":
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid_grant"})
			return
		}
		claims := jwtv5.MapClaims{"sub": sub, "aud": audience, "azp": "ruto", "exp": time.Now().Add(5 * time.Minute).Unix()}
		if sub == "alice" {
			claims["resource_access"] = map[string]any{audience: map[string]any{"roles": []string{"manager"}}}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": p.sign(claims), "token_type": "Bearer", "expires_in": 300})
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	p.issuer = server.URL + "/realms/corp"
	return p
}

func (p *appTokenProvider) sign(claims jwtv5.MapClaims) string {
	tokenObj := jwtv5.NewWithClaims(jwtv5.SigningMethodRS256, claims)
	tokenObj.Header["kid"] = p.kid
	token, err := tokenObj.SignedString(p.key)
	require.NoError(p.t, err)
	return token
}

// shared is a shared token of sub; jti keeps tokens of different checks apart
// (the answer to an exchange of one token is given again for a while).
func (p *appTokenProvider) shared(sub string, ttl time.Duration) string {
	return p.sign(jwtv5.MapClaims{"sub": sub, "aud": "ruto", "exp": time.Now().Add(ttl).Unix(), "jti": randomJti()})
}

func randomJti() string {
	buf := make([]byte, 8)
	_, _ = rand.Read(buf)
	return base64.RawURLEncoding.EncodeToString(buf)
}

func setupAppTokenProvider(t *testing.T) *appTokenProvider {
	p := newAppTokenProvider(t)
	jwk.Ins().SetProviders([]rootModel.RootJwt{{Name: "corp", JwkUrl: p.issuer + "/certs"}})
	t.Cleanup(func() { jwk.Ins().SetProviders(nil) })
	require.Eventually(t, func() bool {
		key, _ := jwk.Ins().GetPublicKey("corp", p.kid)
		return key != nil
	}, 2*time.Second, 20*time.Millisecond)
	apptoken.Ins().SetProviders(&rootModel.Root{
		BaseUrl: "https://api.example.com",
		Jwt: []rootModel.RootJwt{{
			Name:           "corp",
			Issuer:         p.issuer,
			Client:         &authModel.JwtClient{Id: "ruto", Secret: "s"},
			ExchangeScopes: []string{"roles"},
		}},
	})
	t.Cleanup(func() { apptoken.Ins().SetProviders(&rootModel.Root{}) })
	return p
}

// ordersEndpoint is an endpoint of the orders app: with exchange, the app has
// the audience "orders" and the method wants the manager role of it.
func ordersEndpoint(exchange bool) *domEndpointModel.Endpoint {
	method := &authModel.AuthMethodJWT{Provider: "corp", CookieName: "ruto_corp"}
	if exchange {
		method.Roles = []string{"orders:manager"}
		method.Exchange = &authModel.JwtExchange{
			Audience: "orders",
			Cookie:   authModel.JwtCookie{Name: "ruto_corp_orders", Path: "/orders"},
			Copies:   []authModel.JwtCookie{{Name: "ruto_corp_orders-api", Path: "/orders-api"}},
		}
	}
	return &domEndpointModel.Endpoint{Auth: authModel.Auth{Enabled: true, Methods: []*authModel.AuthMethod{{JWT: method}}}}
}

type backendSeen struct {
	mu      sync.Mutex
	headers []http.Header
}

func (b *backendSeen) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b.mu.Lock()
		b.headers = append(b.headers, r.Header.Clone())
		b.mu.Unlock()
		w.WriteHeader(http.StatusOK)
	})
}

func (b *backendSeen) last() http.Header {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.headers) == 0 {
		return nil
	}
	return b.headers[len(b.headers)-1]
}

func serve(handler http.Handler, authorization, cookie string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/orders/hello", nil)
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	if cookie != "" {
		req.Header.Set("Cookie", cookie)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func responseCookies(rec *httptest.ResponseRecorder) map[string]*http.Cookie {
	result := map[string]*http.Cookie{}
	for _, c := range rec.Result().Cookies() {
		result[c.Name] = c
	}
	return result
}

func tokenAud(t *testing.T, token string) []string {
	parsed, _, err := jwtv5.NewParser().ParseUnverified(token, jwtv5.MapClaims{})
	require.NoError(t, err)
	aud, err := parsed.Claims.GetAudience()
	require.NoError(t, err)
	return aud
}

func TestAuthAppToken(t *testing.T) {
	p := setupAppTokenProvider(t)
	backend := &backendSeen{}
	handler := NewAuth(ordersEndpoint(true))(backend.handler())

	// The shared token only: exchanged for the app token, which the backend
	// gets instead of the shared one and the browser keeps in the cookies of
	// both apps with this audience.
	shared := p.shared("alice", time.Hour)
	rec := serve(handler, "", "ruto_corp="+shared+"; theme=dark")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, int32(1), p.exchanges.Load())
	seen := backend.last()
	appToken := strings.TrimPrefix(seen.Get("Authorization"), "Bearer ")
	assert.Equal(t, []string{"orders"}, tokenAud(t, appToken))
	assert.Equal(t, "theme=dark", seen.Get("Cookie"), "the shared cookie does not reach the backend")
	cookies := responseCookies(rec)
	require.Contains(t, cookies, "ruto_corp_orders")
	require.Contains(t, cookies, "ruto_corp_orders-api")
	own := cookies["ruto_corp_orders"]
	assert.Equal(t, appToken, own.Value)
	assert.Equal(t, "/orders", own.Path)
	assert.Equal(t, appToken, cookies["ruto_corp_orders-api"].Value)
	assert.Equal(t, "/orders-api", cookies["ruto_corp_orders-api"].Path)
	assert.InDelta(t, 300, own.MaxAge, 5)
	assert.True(t, own.HttpOnly)
	assert.True(t, own.Secure, "the gateway is on https")
	assert.Equal(t, http.SameSiteLaxMode, own.SameSite)

	// With the app token cookie: no exchange, the cookie stays for the backend.
	rec = serve(handler, "", "ruto_corp="+shared+"; ruto_corp_orders="+appToken)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, int32(1), p.exchanges.Load())
	assert.Empty(t, rec.Result().Cookies())
	assert.Equal(t, "Bearer "+appToken, backend.last().Get("Authorization"))
	assert.Equal(t, "ruto_corp_orders="+appToken, backend.last().Get("Cookie"))

	// An app token about to expire is exchanged anew.
	dying := p.sign(jwtv5.MapClaims{"sub": "alice", "aud": "orders", "exp": time.Now().Add(2 * time.Second).Unix(),
		"resource_access": map[string]any{"orders": map[string]any{"roles": []string{"manager"}}}})
	rec = serve(handler, "", "ruto_corp="+p.shared("alice", time.Hour)+"; ruto_corp_orders="+dying)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, int32(2), p.exchanges.Load())
	assert.NotEqual(t, dying, responseCookies(rec)["ruto_corp_orders"].Value)

	// An app token for another audience is not taken.
	foreign := p.sign(jwtv5.MapClaims{"sub": "alice", "aud": "shop", "exp": time.Now().Add(time.Minute).Unix(),
		"resource_access": map[string]any{"orders": map[string]any{"roles": []string{"manager"}}}})
	rec = serve(handler, "", "ruto_corp="+p.shared("alice", time.Hour)+"; ruto_corp_orders="+foreign)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, int32(3), p.exchanges.Load())

	// A token of the provider in the header is taken as it is, no exchange.
	rec = serve(handler, "Bearer "+appToken, "")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, int32(3), p.exchanges.Load())
	assert.Equal(t, "Bearer "+appToken, backend.last().Get("Authorization"))
}

func TestAuthAppTokenRefusals(t *testing.T) {
	p := setupAppTokenProvider(t)
	backend := &backendSeen{}
	handler := NewAuth(ordersEndpoint(true))(backend.handler())

	tests := []struct {
		name          string
		cookie        string
		wantCode      int
		wantChallenge bool
	}{
		{"no session", "", http.StatusUnauthorized, true},
		{"expired shared token", "ruto_corp=" + p.shared("alice", -time.Minute), http.StatusUnauthorized, true},
		{"shared token rejected by the provider", "ruto_corp=" + p.shared("gone", time.Hour), http.StatusUnauthorized, true},
		{"user without access to the audience", "ruto_corp=" + p.shared("bob", time.Hour), http.StatusForbidden, false},
		{"app token without the role", "ruto_corp=" + p.shared("carol", time.Hour), http.StatusForbidden, false},
		{"provider is down", "ruto_corp=" + p.shared("down", time.Hour), http.StatusBadGateway, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := serve(handler, "", tt.cookie)
			require.Equal(t, tt.wantCode, rec.Code)
			if tt.wantChallenge {
				assert.Equal(t, `Bearer error="invalid_token"`, rec.Header().Get("WWW-Authenticate"))
			} else {
				assert.Empty(t, rec.Header().Get("WWW-Authenticate"))
			}
		})
	}
	assert.Nil(t, backend.last(), "no request reached the backend")

	// carol got an app token, just without the role: it is kept all the same.
	rec := serve(handler, "", "ruto_corp="+p.shared("carol", time.Hour))
	require.Equal(t, http.StatusForbidden, rec.Code)
	assert.Contains(t, responseCookies(rec), "ruto_corp_orders")
}

func TestAuthAppTokenSimultaneousRequests(t *testing.T) {
	p := setupAppTokenProvider(t)
	p.delay = 100 * time.Millisecond
	backend := &backendSeen{}
	handler := NewAuth(ordersEndpoint(true))(backend.handler())

	shared := p.shared("alice", time.Hour)
	var wg sync.WaitGroup
	codes := make([]int, 3)
	for i := range codes {
		wg.Go(func() { codes[i] = serve(handler, "", "ruto_corp="+shared).Code })
	}
	wg.Wait()
	assert.Equal(t, []int{http.StatusOK, http.StatusOK, http.StatusOK}, codes)
	assert.Equal(t, int32(1), p.exchanges.Load())
}

func TestAuthRutoBearerIsNotCredentials(t *testing.T) {
	p := setupAppTokenProvider(t)
	gwAuthModel.SetRutoKids([]string{"ruto-k1"})
	t.Cleanup(func() { gwAuthModel.SetRutoKids(nil) })

	rutoKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	rutoObj := jwtv5.NewWithClaims(jwtv5.SigningMethodRS256, jwtv5.MapClaims{"sub": "alice", "exp": time.Now().Add(time.Hour).Unix()})
	rutoObj.Header["kid"] = "ruto-k1"
	rutoToken, err := rutoObj.SignedString(rutoKey)
	require.NoError(t, err)

	// An app with an audience: the session is in the cookies, the backend
	// gets the app token in place of the gateway's own one.
	backend := &backendSeen{}
	shared := p.shared("alice", time.Hour)
	rec := serve(NewAuth(ordersEndpoint(true))(backend.handler()), "Bearer "+rutoToken, "ruto_corp="+shared)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, []string{"orders"}, tokenAud(t, strings.TrimPrefix(backend.last().Get("Authorization"), "Bearer ")))

	// An app without one gets the shared token.
	rec = serve(NewAuth(ordersEndpoint(false))(backend.handler()), "Bearer "+rutoToken, "ruto_corp="+shared)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "Bearer "+shared, backend.last().Get("Authorization"))
	assert.Equal(t, "ruto_corp="+shared, backend.last().Get("Cookie"), "without an audience the shared cookie stays")

	// The gateway's own token alone lets nobody in.
	rec = serve(NewAuth(ordersEndpoint(false))(backend.handler()), "Bearer "+rutoToken, "")
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}
