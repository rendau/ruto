package authflow

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	jwtv5 "github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appModel "github.com/rendau/ruto/internal/domain/app/model"
	authModel "github.com/rendau/ruto/internal/domain/auth/model"
	rootModel "github.com/rendau/ruto/internal/domain/root/model"
	snapshotModel "github.com/rendau/ruto/internal/domain/snapshot/model"
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
	// or none at all; on a refresh, another subject.
	idAud     string
	idNonce   string
	idSub     string
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
	exp := time.Now().Add(5 * time.Minute).Unix()
	access := op.sign(jwtv5.MapClaims{"sub": "u1", "iss": op.issuer, "gen": 2, "exp": exp})
	if strings.HasPrefix(refreshToken, "refresh-1") {
		// The gateway answers a repeated refresh of one token from its cache,
		// so a test that needs a fresh exchange uses a "refresh-1…" variant.
		refreshToken = "refresh-1"
	}
	switch refreshToken {
	case "refresh-1":
		// Keycloak sends a fresh id token along with the rotated pair.
		aud, sub := op.lastUser, "u1"
		if op.idAud != "" {
			aud = op.idAud
		}
		if op.idSub != "" {
			sub = op.idSub
		}
		op.lastIdToken = ""
		if !op.noIdToken {
			op.lastIdToken = op.sign(jwtv5.MapClaims{"sub": sub, "iss": op.issuer, "exp": exp, "aud": aud, "azp": aud})
		}
		_ = json.NewEncoder(w).Encode(oidc.TokenResponse{
			AccessToken: access, RefreshToken: "refresh-2", IdToken: op.lastIdToken, ExpiresIn: 300, RefreshExpiresIn: 1700,
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

// newUnits builds the login units of snapshot, as the gateway does on a new
// snapshot, with a cookie key.
func newUnits(t *testing.T, snapshot *rootModel.Root) map[string]*Unit {
	SetCookieKey(snapshotModel.CookieKeyFrom("test-admin-secret"))
	t.Cleanup(func() { SetCookieKey(nil) })
	require.NoError(t, snapshot.Normalize())
	snapshot.InheritDown()
	return NewUnits(snapshot)
}

func newCorp(t *testing.T, op *fakeOP, client *authModel.JwtClient) *Unit {
	snapshot := rootModel.NewEmpty()
	snapshot.Jwt = []rootModel.RootJwt{op.provider(t, client)}
	u := newUnits(t, snapshot)["corp"]
	require.NotNil(t, u)
	return u
}

func newRequest(target string, cookies ...*http.Cookie) *http.Request {
	req := httptest.NewRequest(http.MethodGet, target, nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	return req
}

// login starts a login for ret and returns the answer, the query the browser
// takes to the provider and the login cookie; the OP learns the challenge and
// the nonce, as a real one does from that query.
func login(t *testing.T, op *fakeOP, u *Unit, ret string, opts *LoginOptions) (*httptest.ResponseRecorder, url.Values, *http.Cookie) {
	t.Helper()
	rec := httptest.NewRecorder()
	require.NoError(t, u.Login(rec, newRequest("/_auth/corp/crm/auth"), ret, opts))
	require.Equal(t, http.StatusFound, rec.Code, rec.Body.String())
	authUrl, err := url.Parse(rec.Header().Get("Location"))
	require.NoError(t, err)
	q := authUrl.Query()
	op.challenge = q.Get("code_challenge")
	op.nonce = q.Get("nonce")
	return rec, q, cookieByName(t, rec, u.loginCookieName(q.Get("state")))
}

// callback brings the provider's answer to the gateway.
func callback(u *Unit, query string, cookies ...*http.Cookie) (*httptest.ResponseRecorder, string, error) {
	rec := httptest.NewRecorder()
	ret, err := u.Callback(rec, newRequest("/_auth/corp/callback?"+query, cookies...))
	return rec, ret, err
}

func loginErrorCode(t *testing.T, err error) string {
	t.Helper()
	failure, ok := errors.AsType[*LoginError](err)
	require.True(t, ok, "%v is not a login error", err)
	return failure.Code
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

func noSessionCookies(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	for _, c := range rec.Result().Cookies() {
		assert.NotEqual(t, "ruto_corp", c.Name, "no session")
		assert.NotEqual(t, "ruto-rt_corp", c.Name, "no session")
	}
}

const testRet = "/_auth/corp/crm/realms/corp/auth?client_id=crm&state=front-state"

func TestLoginFlow(t *testing.T) {
	op := newFakeOP(t)
	u := newCorp(t, op, &authModel.JwtClient{Id: "ruto", Secret: "s3cret", Scopes: []string{"profile"}})

	// login: a redirect to the provider and a login cookie of its own
	rec, q, loginCookie := login(t, op, u, testRet, nil)
	authUrl, err := url.Parse(rec.Header().Get("Location"))
	require.NoError(t, err)
	assert.Equal(t, op.issuer+"/auth", authUrl.Scheme+"://"+authUrl.Host+authUrl.Path)
	assert.Equal(t, "code", q.Get("response_type"))
	assert.Equal(t, "ruto", q.Get("client_id"))
	assert.Equal(t, publicUrl+"/_auth/corp/callback", q.Get("redirect_uri"))
	assert.Equal(t, "openid profile", q.Get("scope"))
	assert.Equal(t, "S256", q.Get("code_challenge_method"))
	require.NotEmpty(t, q.Get("state"))
	require.NotEmpty(t, op.challenge)
	require.NotEmpty(t, op.nonce)
	for _, name := range []string{"prompt", "max_age", "login_hint", "ui_locales", "acr_values"} {
		assert.NotContains(t, q, name, "nothing asked, nothing sent")
	}

	assert.Equal(t, "__Host-ruto-login_corp_"+q.Get("state"), loginCookie.Name)
	assert.True(t, loginCookie.HttpOnly)
	assert.True(t, loginCookie.Secure)
	assert.Equal(t, "/", loginCookie.Path)
	assert.Equal(t, http.SameSiteLaxMode, loginCookie.SameSite)
	assert.Equal(t, loginCookieMaxAge, loginCookie.MaxAge)

	// another state finds no login of its own; the provider is not asked
	_, ret, err := callback(u, "state=wrong&code=good-code", loginCookie)
	assert.Equal(t, "invalid_request", loginErrorCode(t, err))
	assert.Empty(t, ret, "the gateway does not know where the person came from")
	assert.Equal(t, 0, op.tokenCalls)

	// the login cookie is gone (another tab finished, it expired)
	_, ret, err = callback(u, "state="+q.Get("state")+"&code=good-code")
	assert.Equal(t, "invalid_request", loginErrorCode(t, err))
	assert.Empty(t, ret)

	// a cookie that says another state than its name
	forged := &http.Cookie{Name: loginCookie.Name, Value: strings.Replace(loginCookie.Value, q.Get("state"), "other", 1)}
	_, ret, err = callback(u, "state="+q.Get("state")+"&code=good-code", forged)
	assert.Equal(t, "state_mismatch", loginErrorCode(t, err))
	assert.Equal(t, testRet, ret)
	assert.Equal(t, 0, op.tokenCalls)

	// the provider refused the code
	rec, ret, err = callback(u, "state="+q.Get("state")+"&code=bad", loginCookie)
	assert.Equal(t, "invalid_grant", loginErrorCode(t, err))
	assert.Equal(t, testRet, ret)
	noSessionCookies(t, rec)

	// the real thing
	rec, ret, err = callback(u, "state="+q.Get("state")+"&code=good-code", loginCookie)
	require.NoError(t, err)
	assert.Equal(t, testRet, ret, "back to the request the login was started for")
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
	session := decodeSessionCookie(refresh.Name, refresh.Value)
	assert.InDelta(t, time.Now().Unix()+1800, session.Expires, 2, "the session knows when it runs out")
	session.Expires = 0
	assert.Equal(t, sessionCookie{RefreshToken: "refresh-1", IdToken: op.lastIdToken}, session)
	assert.Equal(t, 1800, refresh.MaxAge)

	cleared := cookieByName(t, rec, loginCookie.Name)
	assert.Equal(t, -1, cleared.MaxAge, "the login cookie is dropped")

	// the session is there for the broker
	req := newRequest("/_auth/corp/crm/auth", access, refresh)
	got, err := u.Session(httptest.NewRecorder(), req)
	require.NoError(t, err)
	assert.Equal(t, "u1", got.Claims["sub"])

	// logout: cookies cleared, provider session ended with the id token
	rec = httptest.NewRecorder()
	u.Logout(rec, newRequest("/_auth/corp/crm/logout", access, refresh), "https://crm.example.test/bye", "")
	require.Equal(t, http.StatusFound, rec.Code)
	endUrl, err := url.Parse(rec.Header().Get("Location"))
	require.NoError(t, err)
	assert.Equal(t, op.issuer+"/logout", endUrl.Scheme+"://"+endUrl.Host+endUrl.Path)
	assert.Equal(t, op.lastIdToken, endUrl.Query().Get("id_token_hint"))
	assert.Equal(t, "https://crm.example.test/bye", endUrl.Query().Get("post_logout_redirect_uri"))
	assert.Equal(t, "ruto", endUrl.Query().Get("client_id"))
	assert.Equal(t, -1, cookieByName(t, rec, "ruto_corp").MaxAge)
	assert.Equal(t, -1, cookieByName(t, rec, "ruto-rt_corp").MaxAge)
}

func TestLoginOptionsGoToTheProvider(t *testing.T) {
	op := newFakeOP(t)
	u := newCorp(t, op, &authModel.JwtClient{Id: "ruto", Secret: "s3cret"})

	_, q, _ := login(t, op, u, testRet, &LoginOptions{
		Prompt:    "login consent",
		MaxAge:    "0",
		LoginHint: "alice@example.test",
		UiLocales: "ru en",
		AcrValues: "urn:mace:incommon:iap:silver",
	})
	assert.Equal(t, "login consent", q.Get("prompt"))
	assert.Equal(t, "0", q.Get("max_age"), "zero is a value, not an absence")
	assert.Equal(t, "alice@example.test", q.Get("login_hint"))
	assert.Equal(t, "ru en", q.Get("ui_locales"))
	assert.Equal(t, "urn:mace:incommon:iap:silver", q.Get("acr_values"))

	_, q, _ = login(t, op, u, testRet, &LoginOptions{LoginHint: "bob"})
	assert.Equal(t, "bob", q.Get("login_hint"))
	assert.NotContains(t, q, "prompt")
	assert.NotContains(t, q, "max_age")
}

func TestParallelLogins(t *testing.T) {
	op := newFakeOP(t)
	u := newCorp(t, op, &authModel.JwtClient{Id: "ruto", Secret: "s3cret"})

	// two tabs start a login one after the other; the browser keeps both cookies
	const retA, retB = testRet + "&tab=a", testRet + "&tab=b"
	_, qA, cookieA := login(t, op, u, retA, nil)
	challengeA, nonceA := op.challenge, op.nonce
	_, qB, cookieB := login(t, op, u, retB, nil)
	require.NotEqual(t, qA.Get("state"), qB.Get("state"))
	require.NotEqual(t, cookieA.Name, cookieB.Name, "the second login does not overwrite the first")

	// the second tab comes back first
	rec, ret, err := callback(u, "state="+qB.Get("state")+"&code=good-code", cookieA, cookieB)
	require.NoError(t, err)
	assert.Equal(t, retB, ret)
	assert.Equal(t, -1, cookieByName(t, rec, cookieB.Name).MaxAge)
	for _, c := range rec.Result().Cookies() {
		assert.NotEqual(t, cookieA.Name, c.Name, "the other login is left alone")
	}

	// then the first: its cookie is still there and its state matches
	op.challenge, op.nonce = challengeA, nonceA
	rec, ret, err = callback(u, "state="+qA.Get("state")+"&code=good-code", cookieA)
	require.NoError(t, err)
	assert.Equal(t, retA, ret)
	cookieByName(t, rec, "ruto_corp")
	assert.Equal(t, 2, op.tokenCalls)
}

func sessionCookieOf(name, path string, session sessionCookie) *http.Cookie {
	value, ok := session.encode(name)
	if !ok {
		panic("no cookie key in the test")
	}
	return &http.Cookie{Name: name, Path: path, Value: value}
}

// renewWith asks for the session with only a refresh cookie: the access
// token is renewed by it.
func renewWith(u *Unit, refresh *http.Cookie) (*httptest.ResponseRecorder, error) {
	rec := httptest.NewRecorder()
	_, err := u.Session(rec, newRequest("/_auth/corp/crm/token", refresh))
	return rec, err
}

func TestRenew(t *testing.T) {
	op := newFakeOP(t)
	u := newCorp(t, op, &authModel.JwtClient{Id: "ruto", Secret: "s3cret"})

	_, err := u.Session(httptest.NewRecorder(), newRequest("/x"))
	assert.ErrorIs(t, err, ErrNoSession, "no cookies")
	assert.Equal(t, 0, op.tokenCalls)

	// a rotated pair: new access cookie, new refresh token, the fresh id token
	// replaces the one from login
	loginIdToken := op.sign(jwtv5.MapClaims{"sub": "u1", "iss": op.issuer, "exp": time.Now().Add(5 * time.Minute).Unix(), "aud": "ruto"})
	rec, err := renewWith(u, sessionCookieOf("ruto-rt_corp", "/_auth/corp", sessionCookie{RefreshToken: "refresh-1", IdToken: loginIdToken}))
	require.NoError(t, err)
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
	require.NotEmpty(t, op.lastIdToken)
	renewed := decodeSessionCookie(refresh.Name, refresh.Value)
	renewed.Expires = 0
	assert.Equal(t, sessionCookie{RefreshToken: "refresh-2", IdToken: op.lastIdToken}, renewed)

	// a renewed id token for another user or another client is refused, the
	// cookies are left alone
	for _, tc := range []struct {
		name, refreshToken string
		setup              func()
	}{
		{"another subject", "refresh-1-sub", func() { op.idSub = "u2" }},
		{"another audience", "refresh-1-aud", func() { op.idAud = "other-client" }},
	} {
		op.idSub, op.idAud = "", ""
		tc.setup()
		rec, err = renewWith(u, sessionCookieOf("ruto-rt_corp", "/_auth/corp", sessionCookie{RefreshToken: tc.refreshToken, IdToken: loginIdToken}))
		assert.ErrorIs(t, err, ErrUnavailable, tc.name)
		assert.Empty(t, rec.Header().Values("Set-Cookie"), tc.name)
	}
	op.idSub, op.idAud = "", ""

	// a provider that does not rotate: the refresh token stays, the session
	// length falls back to the default
	rec, err = renewWith(u, sessionCookieOf("ruto-rt_corp", "/_auth/corp", sessionCookie{RefreshToken: "sticky", IdToken: "id-token-1"}))
	require.NoError(t, err)
	refresh = cookieByName(t, rec, "ruto-rt_corp")
	sticky := decodeSessionCookie(refresh.Name, refresh.Value)
	assert.InDelta(t, time.Now().Unix()+defaultSessionMaxAge, sticky.Expires, 2)
	sticky.Expires = 0
	assert.Equal(t, sessionCookie{RefreshToken: "sticky", IdToken: "id-token-1"}, sticky)
	assert.Equal(t, defaultSessionMaxAge, refresh.MaxAge)

	// the session is over, and the cookies are not touched: a parallel
	// refresh may have just renewed them
	rec, err = renewWith(u, sessionCookieOf("ruto-rt_corp", "/_auth/corp", sessionCookie{RefreshToken: "refresh-old"}))
	assert.ErrorIs(t, err, ErrNoSession)
	assert.Empty(t, rec.Header().Values("Set-Cookie"))

	// the provider is in trouble: not the user's fault, the session stays
	rec, err = renewWith(u, sessionCookieOf("ruto-rt_corp", "/_auth/corp", sessionCookie{RefreshToken: "unavailable"}))
	assert.ErrorIs(t, err, ErrUnavailable)
	assert.Empty(t, rec.Header().Values("Set-Cookie"))
}

func TestLoginFailures(t *testing.T) {
	op := newFakeOP(t)
	u := newCorp(t, op, &authModel.JwtClient{Id: "ruto", Secret: "s3cret"})

	// the provider said no (the person cancelled, or has no access): its own
	// code with the ret of the login, the login cookie is dropped
	_, q, loginCookie := login(t, op, u, testRet, nil)
	rec, ret, err := callback(u, "error=access_denied&error_description=User+cancelled&state="+q.Get("state"), loginCookie)
	assert.Equal(t, "access_denied", loginErrorCode(t, err))
	assert.Equal(t, testRet, ret)
	assert.Less(t, cookieByName(t, rec, loginCookie.Name).MaxAge, 0, "login cookie dropped")
	assert.Equal(t, 0, op.tokenCalls)

	// a code that does not look like one is not echoed into an address
	_, q, loginCookie = login(t, op, u, testRet, nil)
	_, _, err = callback(u, "error="+url.QueryEscape("<script>alert(1)</script>")+"&state="+q.Get("state"), loginCookie)
	assert.Equal(t, "server_error", loginErrorCode(t, err))

	// no login cookie: the gateway does not know where the person came from
	_, ret, err = callback(u, "error=access_denied&state=x")
	assert.Equal(t, "access_denied", loginErrorCode(t, err))
	assert.Empty(t, ret)

	// the provider is down during the code exchange
	_, q, loginCookie = login(t, op, u, testRet, nil)
	op.server.Close()
	_, ret, err = callback(u, "state="+q.Get("state")+"&code=good-code", loginCookie)
	assert.Equal(t, "temporarily_unavailable", loginErrorCode(t, err))
	assert.Equal(t, testRet, ret)
}

func TestLoginProviderUnavailable(t *testing.T) {
	snapshot := rootModel.NewEmpty()
	snapshot.Jwt = []rootModel.RootJwt{{
		Name:      "corp",
		Issuer:    "http://127.0.0.1:1/realms/corp", // nobody listens there
		PublicUrl: publicUrl,
		Client:    &authModel.JwtClient{Id: "ruto", Secret: "s"},
	}}
	u := newUnits(t, snapshot)["corp"]

	rec := httptest.NewRecorder()
	err := u.Login(rec, newRequest("/_auth/corp/crm/auth"), testRet, nil)
	assert.ErrorIs(t, err, ErrUnavailable)
	assert.Empty(t, rec.Header().Values("Set-Cookie"), "nothing written, the caller answers")
	assert.Empty(t, rec.Header().Get("Location"))
}

func TestLogoutDropsAppTokens(t *testing.T) {
	op := newFakeOP(t)
	snapshot := rootModel.NewEmpty()
	snapshot.Jwt = []rootModel.RootJwt{op.provider(t, &authModel.JwtClient{Id: "ruto", Secret: "s3cret"})}
	newApp := func(name, prefix string, active bool, jwt ...appModel.AppJwt) *appModel.App {
		return &appModel.App{Active: active, Name: name, PathPrefix: prefix, Backend: appModel.Backend{Url: "http://" + name + ":8080"}, Jwt: jwt}
	}
	snapshot.Apps = []*appModel.App{
		newApp("orders", "orders", true, appModel.AppJwt{Provider: "corp", Audience: "orders"}),
		newApp("orders-api", "orders/api", true, appModel.AppJwt{Provider: "corp", Audience: "orders"}),
		newApp("old", "old", false, appModel.AppJwt{Provider: "corp", Audience: "old"}),
		newApp("shop", "shop", true),
	}
	u := newUnits(t, snapshot)["corp"]

	rec := httptest.NewRecorder()
	u.Logout(rec, newRequest("/_auth/corp/crm/logout"), publicUrl+"/bye", "")
	require.Equal(t, http.StatusFound, rec.Code, rec.Body.String())
	cleared := map[string]string{}
	for _, c := range rec.Result().Cookies() {
		assert.Equal(t, -1, c.MaxAge, c.Name)
		cleared[c.Name] = c.Path
	}
	assert.Equal(t, map[string]string{
		"ruto_corp":            "/",
		"ruto-rt_corp":         "/_auth/corp",
		"ruto_corp_orders":     "/orders",
		"ruto_corp_orders.api": "/orders/api",
	}, cleared)
	assert.Equal(t, publicUrl+"/bye", rec.Header().Get("Location"), "no session, straight to the return address")
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
			u := newCorp(t, op, &authModel.JwtClient{Id: "ruto", Secret: "s"})

			_, q, loginCookie := login(t, op, u, testRet, nil)
			rec, ret, err := callback(u, "state="+q.Get("state")+"&code=good-code", loginCookie)
			assert.Equal(t, "invalid_token", loginErrorCode(t, err))
			assert.Equal(t, testRet, ret)
			assert.Equal(t, 1, op.tokenCalls)
			noSessionCookies(t, rec)
		})
	}
}

func TestNoCookieKeyNoRefreshCookie(t *testing.T) {
	op := newFakeOP(t)
	u := newCorp(t, op, &authModel.JwtClient{Id: "ruto", Secret: "s"})
	SetCookieKey(nil) // an old core that sends no key

	// the login still works: the access cookie is set, the refresh cookie is not
	_, q, loginCookie := login(t, op, u, testRet, nil)
	rec, _, err := callback(u, "state="+q.Get("state")+"&code=good-code", loginCookie)
	require.NoError(t, err)
	cookieByName(t, rec, "ruto_corp")
	for _, c := range rec.Result().Cookies() {
		assert.NotEqual(t, "ruto-rt_corp", c.Name, "no key, no refresh cookie")
	}

	// a refresh cookie from before (another key) is an empty session
	SetCookieKey(snapshotModel.CookieKeyFrom("old-key"))
	stale := sessionCookieOf("ruto-rt_corp", "/_auth/corp", sessionCookie{RefreshToken: "refresh-1"})
	SetCookieKey(snapshotModel.CookieKeyFrom("new-key"))
	_, err = renewWith(u, stale)
	assert.ErrorIs(t, err, ErrNoSession, "time to log in again")
	assert.Equal(t, 1, op.tokenCalls, "the provider is not asked")
}

func TestHttpGatewayGetsPlainCookies(t *testing.T) {
	op := newFakeOP(t)
	snapshot := rootModel.NewEmpty()
	p := op.provider(t, &authModel.JwtClient{Id: "ruto", Secret: "s"})
	p.PublicUrl = "http://localhost:8088"
	snapshot.Jwt = []rootModel.RootJwt{p}
	u := newUnits(t, snapshot)["corp"]

	_, q, loginCookie := login(t, op, u, testRet, nil)
	assert.Equal(t, "ruto-login_corp_"+q.Get("state"), loginCookie.Name)
	assert.False(t, loginCookie.Secure, "no Secure on plain http, or the browser drops the cookie")
	assert.Equal(t, "http://localhost:8088/_auth/corp/callback", q.Get("redirect_uri"))
}

func TestLogoutPassesState(t *testing.T) {
	op := newFakeOP(t)
	u := newCorp(t, op, &authModel.JwtClient{Id: "ruto", Secret: "s3cret"})

	// with the provider's id token: the provider gets the state to pass on
	rec := httptest.NewRecorder()
	u.Logout(rec, newRequest("/_auth/corp/crm/logout",
		sessionCookieOf("ruto-rt_corp", "/_auth/corp", sessionCookie{RefreshToken: "rt", IdToken: "idt"})),
		"https://crm.example.com/bye", "s-1")
	endUrl, err := url.Parse(rec.Header().Get("Location"))
	require.NoError(t, err)
	assert.Equal(t, op.issuer+"/logout", endUrl.Scheme+"://"+endUrl.Host+endUrl.Path)
	assert.Equal(t, "s-1", endUrl.Query().Get("state"))
	assert.Equal(t, "https://crm.example.com/bye", endUrl.Query().Get("post_logout_redirect_uri"))

	// without a session: straight back, the state added here
	rec = httptest.NewRecorder()
	u.Logout(rec, newRequest("/x"), "https://crm.example.com/bye?a=1", "s-1")
	assert.Equal(t, "https://crm.example.com/bye?a=1&state=s-1", rec.Header().Get("Location"))
}
