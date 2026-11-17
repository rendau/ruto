package broker

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"

	"github.com/rendau/ruto/internal/service/gw/handler/http/authflow"
	"github.com/rendau/ruto/internal/service/gw/service/oidc"
)

const providerIssuer = "http://kc.test/realms/corp"

// fakeSessions stands for the provider and the gateway's login session: it
// signs provider tokens with its own key, checks them, exchanges the shared
// token for other audiences.
type fakeSessions struct {
	t   *testing.T
	key *ecdsa.PrivateKey

	mu         sync.Mutex
	session    *authflow.Session
	sessionErr error
	closed     map[string]bool     // provider sessions (sid) that are over
	roles      map[string][]string // client roles of the person by client

	exchangeErr error
	exchanges   atomic.Int32

	renewals      atomic.Int32
	loggedOut     []string // redirect|state of every logout
	userinfoCalls []string // endpoints

	logins    map[string]string // ret of every login started, by state
	loginOpts *authflow.LoginOptions
	loginErr  error // Login fails with it
}

func newFakeSessions(t *testing.T) *fakeSessions {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	f := &fakeSessions{t: t, key: key, roles: map[string][]string{"crm": {"admin"}}, logins: map[string]string{}, closed: map[string]bool{}}
	f.login("u-1")
	return f
}

// login gives the gateway a live session of sub, like after a provider login:
// a provider session (sid) of its own, the earlier one is over.
func (f *fakeSessions) login(sub string) {
	f.mu.Lock()
	if f.session != nil {
		f.closed[f.session.Claims["sid"].(string)] = true
	}
	f.mu.Unlock()
	f.startSession(sub, "kc-"+randomToken(8))
}

// renew gives the session a fresh shared token, in the same provider session.
func (f *fakeSessions) renew(sub string) {
	f.mu.Lock()
	sid := f.session.Claims["sid"].(string)
	f.mu.Unlock()
	f.startSession(sub, sid)
}

func (f *fakeSessions) startSession(sub, sid string) {
	now := time.Now()
	claims := jwt.MapClaims{
		"iss": providerIssuer, "sub": sub, "aud": "ruto", "azp": "ruto",
		"exp": now.Add(5 * time.Minute).Unix(), "iat": now.Unix(), "auth_time": now.Add(-time.Minute).Unix(),
		"sid": sid, "preferred_username": "alice", "email": "alice@example.test",
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.session = &authflow.Session{
		AccessToken:    f.sign(claims),
		Claims:         claims,
		Expires:        now.Add(5 * time.Minute),
		SessionExpires: now.Add(7 * 24 * time.Hour),
		AuthTime:       now.Add(-time.Minute).Unix(),
	}
	f.sessionErr = nil
}

func (f *fakeSessions) logout(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.session != nil {
		f.closed[f.session.Claims["sid"].(string)] = true
	}
	f.session, f.sessionErr = nil, err
}

func (f *fakeSessions) sign(claims jwt.MapClaims) string {
	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	token.Header["kid"] = "op"
	signed, err := token.SignedString(f.key)
	require.NoError(f.t, err)
	return signed
}

func (f *fakeSessions) Session(_ http.ResponseWriter, _ *http.Request) (*authflow.Session, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.session, f.sessionErr
}

// Login sends the browser to the provider with a state of its own and
// remembers the login, as authflow does in its login cookie.
func (f *fakeSessions) Login(w http.ResponseWriter, r *http.Request, ret string, opts *authflow.LoginOptions) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.loginErr != nil {
		return f.loginErr
	}
	state := randomToken(8)
	f.logins[state] = ret
	f.loginOpts = opts
	http.Redirect(w, r, "http://kc.test/realms/corp/protocol/openid-connect/auth?state="+state, http.StatusFound)
	return nil
}

// Callback finishes a login: the provider's error, or a live session of u-1.
func (f *fakeSessions) Callback(_ http.ResponseWriter, r *http.Request) (string, error) {
	query := r.URL.Query()
	f.mu.Lock()
	ret := f.logins[query.Get("state")]
	delete(f.logins, query.Get("state"))
	f.mu.Unlock()
	if code := query.Get("error"); code != "" {
		return ret, &authflow.LoginError{Code: code, Status: http.StatusUnauthorized, Text: "login failed: " + code}
	}
	if ret == "" {
		return "", &authflow.LoginError{Code: "invalid_request", Status: http.StatusBadRequest, Text: "no login in progress"}
	}
	f.login("u-1")
	return ret, nil
}

func (f *fakeSessions) ParseProviderToken(token string) (jwt.MapClaims, error) {
	claims := jwt.MapClaims{}
	_, err := jwt.ParseWithClaims(token, claims, func(*jwt.Token) (any, error) { return &f.key.PublicKey, nil },
		jwt.WithIssuer(providerIssuer), jwt.WithExpirationRequired(), jwt.WithValidMethods([]string{"ES256"}))
	return claims, err
}

func (f *fakeSessions) SetCookie(w http.ResponseWriter, name, value, path string, maxAge int) {
	http.SetCookie(w, &http.Cookie{Name: name, Value: value, Path: path, MaxAge: maxAge, HttpOnly: true, SameSite: http.SameSiteLaxMode})
}

// Exchange gives a token of audience with the person's roles of it.
func (f *fakeSessions) Exchange(_ context.Context, _, subjectToken, audience string) (string, error) {
	f.exchanges.Add(1)
	if f.exchangeErr != nil {
		return "", f.exchangeErr
	}
	subject, err := f.ParseProviderToken(subjectToken)
	require.NoError(f.t, err)
	now := time.Now()
	return f.sign(jwt.MapClaims{
		"iss": providerIssuer, "sub": subject["sub"], "aud": audience, "azp": "ruto",
		"exp": now.Add(5 * time.Minute).Unix(), "iat": now.Unix(), "sid": subject["sid"],
		"preferred_username": subject["preferred_username"], "email": subject["email"],
		"resource_access": map[string]any{audience: map[string]any{"roles": f.roles[audience]}},
	}), nil
}

type fakeUsed struct {
	mu   sync.Mutex
	used map[string]bool
}

func newFakeUsed() *fakeUsed {
	return &fakeUsed{used: map[string]bool{}}
}

func (f *fakeUsed) MarkUsed(_ context.Context, key string, _ time.Duration) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.used[key] {
		return false
	}
	f.used[key] = true
	return true
}

// publicKeysOf parses a JWK set of EC P-256 keys by the members of the
// standard.
func publicKeysOf(t *testing.T, jwks []byte) map[string]*ecdsa.PublicKey {
	t.Helper()
	var set struct {
		Keys []struct {
			Kid string `json:"kid"`
			Crv string `json:"crv"`
			X   string `json:"x"`
			Y   string `json:"y"`
		} `json:"keys"`
	}
	require.NoError(t, json.Unmarshal(jwks, &set))
	result := map[string]*ecdsa.PublicKey{}
	for _, item := range set.Keys {
		require.Equal(t, "P-256", item.Crv)
		x, err := base64.RawURLEncoding.DecodeString(item.X)
		require.NoError(t, err)
		y, err := base64.RawURLEncoding.DecodeString(item.Y)
		require.NoError(t, err)
		key, err := ecdsa.ParseUncompressedPublicKey(elliptic.P256(), append(append([]byte{4}, x...), y...))
		require.NoError(t, err)
		result[item.Kid] = key
	}
	return result
}

// FreshSession renews the shared token, as the gateway does when less than
// half of its life is left.
func (f *fakeSessions) FreshSession(w http.ResponseWriter, r *http.Request) (*authflow.Session, error) {
	f.mu.Lock()
	current, err := f.session, f.sessionErr
	f.mu.Unlock()
	if err != nil {
		return nil, err
	}
	f.renewals.Add(1)
	f.renew(current.Claims["sub"].(string))
	return f.Session(w, r)
}

func (f *fakeSessions) Logout(w http.ResponseWriter, r *http.Request, redirect, state string) {
	f.mu.Lock()
	f.loggedOut = append(f.loggedOut, redirect+"|"+state)
	if f.session != nil {
		f.closed[f.session.Claims["sid"].(string)] = true
	}
	f.session, f.sessionErr = nil, authflow.ErrNoSession
	f.mu.Unlock()
	http.Redirect(w, r, redirect, http.StatusFound)
}

func (f *fakeSessions) ClearCookie(w http.ResponseWriter, name, path string) {
	http.SetCookie(w, &http.Cookie{Name: name, Value: "", Path: path, MaxAge: -1})
}

// Userinfo answers with the claims of the token it was asked with, like a
// provider's userinfo; a token of a provider session that is over is refused.
func (f *fakeSessions) Userinfo(_ context.Context, endpoint, token string) (*oidc.UserinfoResponse, error) {
	f.mu.Lock()
	f.userinfoCalls = append(f.userinfoCalls, endpoint)
	f.mu.Unlock()
	claims, err := f.ParseProviderToken(token)
	f.mu.Lock()
	closed := err == nil && f.closed[claims["sid"].(string)]
	f.mu.Unlock()
	if err != nil || closed {
		return &oidc.UserinfoResponse{Status: http.StatusUnauthorized, ContentType: "text/plain", Body: []byte("invalid token")}, nil
	}
	body, _ := json.Marshal(map[string]any{
		"sub": claims["sub"], "preferred_username": claims["preferred_username"],
		"aud_of_token": claims["aud"], "resource_access": claims["resource_access"],
	})
	return &oidc.UserinfoResponse{Status: http.StatusOK, ContentType: "application/json", Body: body}, nil
}
