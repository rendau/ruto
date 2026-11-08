package authflow

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	jwtv5 "github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appModel "github.com/rendau/ruto/internal/domain/app/model"
	authModel "github.com/rendau/ruto/internal/domain/auth/model"
	rootModel "github.com/rendau/ruto/internal/domain/root/model"
	"github.com/rendau/ruto/internal/service/gw/service/jwk"
	"github.com/rendau/ruto/internal/service/gw/service/oidc"
)

const (
	testKid   = "login-kid"
	publicUrl = "https://api.example.test"
)

// fakeOP is a minimal OpenID provider: discovery, keys and a token endpoint
// that checks the PKCE verifier against the challenge the test saw in the
// login redirect.
type fakeOP struct {
	t           *testing.T
	server      *httptest.Server
	privateKey  *rsa.PrivateKey
	issuer      string
	challenge   string // set by the test from the /login redirect
	nonce       string // same
	tokenCalls  int
	lastForm    url.Values
	lastUser    string
	lastPass    string
	lastIdToken string
	// Misbehaviour knobs for the id token: another audience, another nonce,
	// or none at all.
	idAud     string
	idNonce   string
	noIdToken bool
}

func newFakeOP(t *testing.T) *fakeOP {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	op := &fakeOP{t: t, privateKey: privateKey}
	mux := http.NewServeMux()
	mux.HandleFunc("/realms/corp/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer":                 op.issuer,
			"authorization_endpoint": op.issuer + "/auth",
			"token_endpoint":         op.issuer + "/token",
			"end_session_endpoint":   op.issuer + "/logout",
			"jwks_uri":               op.issuer + "/certs",
		})
	})
	mux.HandleFunc("/realms/corp/certs", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(jwk.ItemSet{Keys: []*jwk.Item{{
			Kty: "RSA",
			Use: "sig",
			Alg: "RS256",
			Kid: testKid,
			N:   base64.RawURLEncoding.EncodeToString(privateKey.N.Bytes()),
			E:   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(privateKey.E)).Bytes()),
		}}})
	})
	mux.HandleFunc("/realms/corp/token", func(w http.ResponseWriter, r *http.Request) {
		op.tokenCalls++
		op.lastUser, op.lastPass, _ = r.BasicAuth()
		require.NoError(t, r.ParseForm())
		op.lastForm = r.PostForm

		if r.PostForm.Get("grant_type") == "refresh_token" {
			op.refreshGrant(w, r.PostForm.Get("refresh_token"))
			return
		}

		sum := sha256.Sum256([]byte(r.PostForm.Get("code_verifier")))
		if base64.RawURLEncoding.EncodeToString(sum[:]) != op.challenge || r.PostForm.Get("code") != "good-code" {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid_grant"})
			return
		}
		exp := time.Now().Add(5 * time.Minute).Unix()
		aud, nonce := op.lastUser, op.nonce
		if op.idAud != "" {
			aud = op.idAud
		}
		if op.idNonce != "" {
			nonce = op.idNonce
		}
		op.lastIdToken = ""
		if !op.noIdToken {
			op.lastIdToken = op.sign(jwtv5.MapClaims{"sub": "u1", "iss": op.issuer, "exp": exp, "aud": aud, "azp": aud, "nonce": nonce})
		}
		_ = json.NewEncoder(w).Encode(oidc.TokenResponse{
			AccessToken:      op.sign(jwtv5.MapClaims{"sub": "u1", "iss": op.issuer, "exp": exp}),
			RefreshToken:     "refresh-1",
			IdToken:          op.lastIdToken,
			ExpiresIn:        300,
			RefreshExpiresIn: 1800,
		})
	})

	op.server = httptest.NewServer(mux)
	t.Cleanup(op.server.Close)
	op.issuer = op.server.URL + "/realms/corp"
	return op
}

// refreshGrant answers a refresh: "refresh-1" is rotated to "refresh-2",
// "sticky" stays valid and is not sent back, "unavailable" is a provider in
// trouble, anything else is a session that is over.
func (op *fakeOP) refreshGrant(w http.ResponseWriter, refreshToken string) {
	access := op.sign(jwtv5.MapClaims{"sub": "u1", "iss": op.issuer, "gen": 2, "exp": time.Now().Add(5 * time.Minute).Unix()})
	switch refreshToken {
	case "refresh-1":
		_ = json.NewEncoder(w).Encode(oidc.TokenResponse{
			AccessToken: access, RefreshToken: "refresh-2", ExpiresIn: 300, RefreshExpiresIn: 1700,
		})
	case "sticky":
		_ = json.NewEncoder(w).Encode(oidc.TokenResponse{AccessToken: access, ExpiresIn: 300})
	case "unavailable":
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "temporarily_unavailable"})
	default:
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid_grant", "error_description": "Token is not active"})
	}
}

func (op *fakeOP) sign(claims jwtv5.MapClaims) string {
	tokenObj := jwtv5.NewWithClaims(jwtv5.SigningMethodRS256, claims)
	tokenObj.Header["kid"] = testKid
	token, err := tokenObj.SignedString(op.privateKey)
	require.NoError(op.t, err)
	return token
}

// provider registers the OP with the key service, as gw.SetConfig does, and
// waits for its keys.
func (op *fakeOP) provider(t *testing.T, client *authModel.JwtClient) rootModel.RootJwt {
	p := rootModel.RootJwt{
		Name:      "corp",
		Issuer:    op.issuer,
		PublicUrl: publicUrl,
		Client:    client,
	}
	jwk.Ins().SetProviders([]rootModel.RootJwt{p})
	t.Cleanup(func() { jwk.Ins().SetProviders(nil) })
	require.Eventually(t, func() bool {
		key, _ := jwk.Ins().GetPublicKey("corp", testKid)
		return key != nil
	}, 3*time.Second, 20*time.Millisecond)
	return p
}

func buildRouter(t *testing.T, snapshot *rootModel.Root) http.Handler {
	require.NoError(t, snapshot.Normalize())
	snapshot.InheritDown()
	router := chi.NewRouter()
	Register(router, snapshot)
	return router
}

func do(router http.Handler, method, target string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func cookieByName(t *testing.T, rec *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	require.Failf(t, "cookie not set", "no cookie %q among %v", name, rec.Header().Values("Set-Cookie"))
	return nil
}

func TestSharedLoginFlow(t *testing.T) {
	op := newFakeOP(t)
	snapshot := rootModel.NewEmpty()
	snapshot.Cors.AllowOrigins = []string{"https://app.example.test", "*"}
	snapshot.Jwt = []rootModel.RootJwt{op.provider(t, &authModel.JwtClient{Id: "ruto", Secret: "s3cret", Scopes: []string{"profile"}})}
	router := buildRouter(t, snapshot)

	// login: a redirect to the provider and the login cookie
	rec := do(router, http.MethodGet, "/_auth/corp/login?redirect=https://app.example.test/home")
	require.Equal(t, http.StatusFound, rec.Code, rec.Body.String())

	authUrl, err := url.Parse(rec.Header().Get("Location"))
	require.NoError(t, err)
	assert.Equal(t, op.issuer+"/auth", authUrl.Scheme+"://"+authUrl.Host+authUrl.Path)
	q := authUrl.Query()
	assert.Equal(t, "code", q.Get("response_type"))
	assert.Equal(t, "ruto", q.Get("client_id"))
	assert.Equal(t, publicUrl+"/_auth/corp/callback", q.Get("redirect_uri"))
	assert.Equal(t, "openid profile", q.Get("scope"))
	assert.Equal(t, "S256", q.Get("code_challenge_method"))
	assert.NotEmpty(t, q.Get("state"))
	op.challenge = q.Get("code_challenge")
	require.NotEmpty(t, op.challenge)
	op.nonce = q.Get("nonce")
	require.NotEmpty(t, op.nonce)

	loginCookie := cookieByName(t, rec, "__Host-ruto-login_corp")
	assert.True(t, loginCookie.HttpOnly)
	assert.True(t, loginCookie.Secure)
	assert.Equal(t, "/", loginCookie.Path)
	assert.Equal(t, http.SameSiteLaxMode, loginCookie.SameSite)
	assert.Equal(t, loginCookieMaxAge, loginCookie.MaxAge)

	// callback with a wrong state is refused before any call to the provider
	rec = do(router, http.MethodGet, "/_auth/corp/callback?state=wrong&code=good-code", loginCookie)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, 0, op.tokenCalls)

	// callback without the login cookie
	rec = do(router, http.MethodGet, "/_auth/corp/callback?state="+q.Get("state")+"&code=good-code")
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	// the provider refused
	rec = do(router, http.MethodGet, "/_auth/corp/callback?state="+q.Get("state")+"&code=bad", loginCookie)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid_grant")

	// the real thing
	rec = do(router, http.MethodGet, "/_auth/corp/callback?state="+q.Get("state")+"&code=good-code", loginCookie)
	require.Equal(t, http.StatusSeeOther, rec.Code, rec.Body.String())
	assert.Equal(t, "https://app.example.test/home", rec.Header().Get("Location"))
	assert.Equal(t, "ruto", op.lastUser)
	assert.Equal(t, "s3cret", op.lastPass)
	assert.Equal(t, "authorization_code", op.lastForm.Get("grant_type"))
	assert.Equal(t, publicUrl+"/_auth/corp/callback", op.lastForm.Get("redirect_uri"))

	access := cookieByName(t, rec, "ruto_corp")
	assert.Equal(t, "/", access.Path)
	assert.True(t, access.HttpOnly)
	assert.True(t, access.Secure)
	assert.Equal(t, 1800, access.MaxAge, "the session lives as long as the refresh token")
	parsed, _, err := jwtv5.NewParser().ParseUnverified(access.Value, jwtv5.MapClaims{})
	require.NoError(t, err)
	assert.Equal(t, "u1", parsed.Claims.(jwtv5.MapClaims)["sub"])

	refresh := cookieByName(t, rec, "ruto-rt_corp")
	assert.Equal(t, "/_auth/corp", refresh.Path)
	assert.Equal(t, sessionCookie{RefreshToken: "refresh-1", IdToken: op.lastIdToken}, decodeSessionCookie(refresh.Value))
	assert.Equal(t, 1800, refresh.MaxAge)

	cleared := cookieByName(t, rec, "__Host-ruto-login_corp")
	assert.Equal(t, -1, cleared.MaxAge, "the login cookie is dropped")

	// logout: cookies cleared, provider session ended with the id token
	rec = do(router, http.MethodGet, "/_auth/corp/logout?redirect=/bye", access, refresh)
	require.Equal(t, http.StatusFound, rec.Code)
	endUrl, err := url.Parse(rec.Header().Get("Location"))
	require.NoError(t, err)
	assert.Equal(t, op.issuer+"/logout", endUrl.Scheme+"://"+endUrl.Host+endUrl.Path)
	assert.Equal(t, op.lastIdToken, endUrl.Query().Get("id_token_hint"))
	assert.Equal(t, publicUrl+"/bye", endUrl.Query().Get("post_logout_redirect_uri"))
	assert.Equal(t, "ruto", endUrl.Query().Get("client_id"))
	assert.Equal(t, -1, cookieByName(t, rec, "ruto_corp").MaxAge)
	assert.Equal(t, -1, cookieByName(t, rec, "ruto-rt_corp").MaxAge)

	// logout without a session goes straight to the return address
	rec = do(router, http.MethodGet, "/_auth/corp/logout")
	require.Equal(t, http.StatusFound, rec.Code)
	assert.Equal(t, publicUrl+"/", rec.Header().Get("Location"))
}

func sessionCookieOf(name, path string, session sessionCookie) *http.Cookie {
	return &http.Cookie{Name: name, Path: path, Value: session.encode()}
}

func TestRefresh(t *testing.T) {
	op := newFakeOP(t)
	snapshot := rootModel.NewEmpty()
	snapshot.Jwt = []rootModel.RootJwt{op.provider(t, &authModel.JwtClient{Id: "ruto", Secret: "s3cret"})}
	router := buildRouter(t, snapshot)
	const target = "/_auth/corp/refresh"

	assert.Equal(t, http.StatusMethodNotAllowed, do(router, http.MethodGet, target).Code, "a cross-site GET must not renew the session")

	rec := do(router, http.MethodPost, target)
	assert.Equal(t, http.StatusUnauthorized, rec.Code, "no refresh cookie")
	assert.Equal(t, 0, op.tokenCalls)
	assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))

	// a rotated pair: new access cookie, new refresh token, the id token kept for logout
	rec = do(router, http.MethodPost, target,
		sessionCookieOf("ruto-rt_corp", "/_auth/corp", sessionCookie{RefreshToken: "refresh-1", IdToken: "id-token-1"}))
	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())
	assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
	assert.Equal(t, "refresh_token", op.lastForm.Get("grant_type"))
	assert.Equal(t, "refresh-1", op.lastForm.Get("refresh_token"))
	assert.Equal(t, "ruto", op.lastUser)
	assert.Equal(t, "s3cret", op.lastPass)

	access := cookieByName(t, rec, "ruto_corp")
	assert.Equal(t, "/", access.Path)
	assert.True(t, access.HttpOnly)
	assert.True(t, access.Secure)
	assert.Equal(t, 1700, access.MaxAge)
	parsed, _, err := jwtv5.NewParser().ParseUnverified(access.Value, jwtv5.MapClaims{})
	require.NoError(t, err)
	assert.EqualValues(t, 2, parsed.Claims.(jwtv5.MapClaims)["gen"])

	refresh := cookieByName(t, rec, "ruto-rt_corp")
	assert.Equal(t, "/_auth/corp", refresh.Path)
	assert.Equal(t, 1700, refresh.MaxAge)
	assert.Equal(t, sessionCookie{RefreshToken: "refresh-2", IdToken: "id-token-1"}, decodeSessionCookie(refresh.Value))

	// a provider that does not rotate: the refresh token stays, the session
	// length falls back to the default
	rec = do(router, http.MethodPost, target,
		sessionCookieOf("ruto-rt_corp", "/_auth/corp", sessionCookie{RefreshToken: "sticky", IdToken: "id-token-1"}))
	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())
	refresh = cookieByName(t, rec, "ruto-rt_corp")
	assert.Equal(t, sessionCookie{RefreshToken: "sticky", IdToken: "id-token-1"}, decodeSessionCookie(refresh.Value))
	assert.Equal(t, defaultSessionMaxAge, refresh.MaxAge)

	// the session is over: 401, and the cookies are not touched, a parallel
	// refresh may have just renewed them
	rec = do(router, http.MethodPost, target,
		sessionCookieOf("ruto-rt_corp", "/_auth/corp", sessionCookie{RefreshToken: "refresh-old"}))
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Empty(t, rec.Header().Values("Set-Cookie"))

	// the provider is in trouble: not the user's fault, the session stays
	rec = do(router, http.MethodPost, target,
		sessionCookieOf("ruto-rt_corp", "/_auth/corp", sessionCookie{RefreshToken: "unavailable"}))
	assert.Equal(t, http.StatusBadGateway, rec.Code)
	assert.Empty(t, rec.Header().Values("Set-Cookie"))
}

func TestReturnUrlIsChecked(t *testing.T) {
	op := newFakeOP(t)
	snapshot := rootModel.NewEmpty()
	snapshot.Cors.AllowOrigins = []string{"https://app.example.test", "https://*.fronts.example"}
	snapshot.Jwt = []rootModel.RootJwt{op.provider(t, &authModel.JwtClient{Id: "ruto", Secret: "s"})}
	router := buildRouter(t, snapshot)

	for _, redirect := range []string{
		"https://evil.example/steal",
		"//evil.example/steal",
		"javascript:alert(1)",
		"https://app.example.test.evil.example/",
		"https://fronts.example/",
		"https://shop.fronts.example.evil.example/",
	} {
		rec := do(router, http.MethodGet, "/_auth/corp/login?redirect="+url.QueryEscape(redirect))
		assert.Equal(t, http.StatusBadRequest, rec.Code, redirect)
		rec = do(router, http.MethodGet, "/_auth/corp/logout?redirect="+url.QueryEscape(redirect))
		assert.Equal(t, http.StatusBadRequest, rec.Code, redirect)
	}

	for _, redirect := range []string{"", "/shop/ui", "https://app.example.test/x?y=1", publicUrl + "/shop", "https://shop.fronts.example/home"} {
		rec := do(router, http.MethodGet, "/_auth/corp/login?redirect="+url.QueryEscape(redirect))
		assert.Equal(t, http.StatusFound, rec.Code, redirect)
	}
}

func TestAppOwnClientUnit(t *testing.T) {
	op := newFakeOP(t)
	snapshot := rootModel.NewEmpty()
	snapshot.Jwt = []rootModel.RootJwt{op.provider(t, nil)} // no default client: no shared login
	snapshot.Apps = []*appModel.App{{
		Active:     true,
		Name:       "shop",
		PathPrefix: "shop/api",
		Backend:    appModel.Backend{Url: "http://shop:8080"},
		Jwt:        []appModel.AppJwt{{Provider: "corp", Client: authModel.JwtClient{Id: "shop-client", Secret: "s"}}},
	}, {
		Active:     true,
		Name:       "orphan",
		PathPrefix: "orphan",
		Backend:    appModel.Backend{Url: "http://orphan:8080"},
		Jwt:        []appModel.AppJwt{{Provider: "nobody", Client: authModel.JwtClient{Id: "x", Secret: "s"}}},
	}}
	router := buildRouter(t, snapshot)

	assert.Equal(t, http.StatusNotFound, do(router, http.MethodGet, "/_auth/corp/login").Code, "no default client, no shared login")
	assert.Equal(t, http.StatusNotFound, do(router, http.MethodGet, "/_auth/nobody/orphan/login").Code, "unknown provider is skipped")

	rec := do(router, http.MethodGet, "/_auth/corp/shop/api/login?redirect=/shop/api/ui")
	require.Equal(t, http.StatusFound, rec.Code, rec.Body.String())
	authUrl, err := url.Parse(rec.Header().Get("Location"))
	require.NoError(t, err)
	q := authUrl.Query()
	assert.Equal(t, "shop-client", q.Get("client_id"))
	assert.Equal(t, publicUrl+"/_auth/corp/shop/api/callback", q.Get("redirect_uri"))
	op.challenge = q.Get("code_challenge")
	op.nonce = q.Get("nonce")
	loginCookie := cookieByName(t, rec, "__Host-ruto-login_corp_shop.api")

	rec = do(router, http.MethodGet, "/_auth/corp/shop/api/callback?state="+q.Get("state")+"&code=good-code", loginCookie)
	require.Equal(t, http.StatusSeeOther, rec.Code, rec.Body.String())
	assert.Equal(t, publicUrl+"/shop/api/ui", rec.Header().Get("Location"))

	access := cookieByName(t, rec, "ruto_corp_shop.api")
	assert.Equal(t, "/shop/api", access.Path, "the app cookie travels only with the app's requests")
	refresh := cookieByName(t, rec, "ruto-rt_corp_shop.api")
	assert.Equal(t, "/_auth/corp/shop/api", refresh.Path)

	// the app's unit renews its own cookies with its own client
	rec = do(router, http.MethodPost, "/_auth/corp/shop/api/refresh", refresh)
	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())
	assert.Equal(t, "shop-client", op.lastUser)
	assert.Equal(t, "/shop/api", cookieByName(t, rec, "ruto_corp_shop.api").Path)
	assert.Equal(t, "/_auth/corp/shop/api", cookieByName(t, rec, "ruto-rt_corp_shop.api").Path)

	// the jwt method of the app reads exactly this cookie
	assert.Equal(t, "ruto_corp_shop.api", snapshot.Apps[0].JwtCookieName("corp"))
}

func TestIdTokenIsChecked(t *testing.T) {
	cases := []struct {
		name  string
		setup func(op *fakeOP)
	}{
		{"no id token", func(op *fakeOP) { op.noIdToken = true }},
		{"another audience", func(op *fakeOP) { op.idAud = "other-client" }},
		{"another nonce", func(op *fakeOP) { op.idNonce = "replayed" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			op := newFakeOP(t)
			tc.setup(op)
			snapshot := rootModel.NewEmpty()
			snapshot.Jwt = []rootModel.RootJwt{op.provider(t, &authModel.JwtClient{Id: "ruto", Secret: "s"})}
			router := buildRouter(t, snapshot)

			rec := do(router, http.MethodGet, "/_auth/corp/login")
			require.Equal(t, http.StatusFound, rec.Code)
			q, err := url.ParseQuery(strings.SplitN(rec.Header().Get("Location"), "?", 2)[1])
			require.NoError(t, err)
			op.challenge = q.Get("code_challenge")
			op.nonce = q.Get("nonce")
			loginCookie := cookieByName(t, rec, "__Host-ruto-login_corp")

			rec = do(router, http.MethodGet, "/_auth/corp/callback?state="+q.Get("state")+"&code=good-code", loginCookie)
			assert.Equal(t, http.StatusBadGateway, rec.Code, rec.Body.String())
			assert.Equal(t, 1, op.tokenCalls)
			for _, c := range rec.Result().Cookies() {
				assert.NotEqual(t, "ruto_corp", c.Name, "no session on a bad id token")
				assert.NotEqual(t, "ruto-rt_corp", c.Name, "no session on a bad id token")
			}
		})
	}
}

func TestHttpGatewayGetsPlainCookies(t *testing.T) {
	op := newFakeOP(t)
	snapshot := rootModel.NewEmpty()
	p := op.provider(t, &authModel.JwtClient{Id: "ruto", Secret: "s"})
	p.PublicUrl = "http://localhost:8088"
	snapshot.Jwt = []rootModel.RootJwt{p}
	router := buildRouter(t, snapshot)

	rec := do(router, http.MethodGet, "/_auth/corp/login")
	require.Equal(t, http.StatusFound, rec.Code)
	loginCookie := cookieByName(t, rec, "ruto-login_corp")
	assert.False(t, loginCookie.Secure, "no Secure on plain http, or the browser drops the cookie")
	assert.False(t, strings.HasPrefix(loginCookie.Name, "__Host-"))
}
