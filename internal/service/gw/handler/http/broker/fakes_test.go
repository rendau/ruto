package broker

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"

	"github.com/rendau/ruto/internal/service/gw/handler/http/authflow"
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
	roles      map[string][]string // client roles of the person by client

	exchangeErr error
	exchanges   atomic.Int32
}

func newFakeSessions(t *testing.T) *fakeSessions {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	f := &fakeSessions{t: t, key: key, roles: map[string][]string{"crm": {"admin"}}}
	f.login("u-1")
	return f
}

// login gives the gateway a live session of sub, like after a provider login.
func (f *fakeSessions) login(sub string) {
	now := time.Now()
	claims := jwt.MapClaims{
		"iss": providerIssuer, "sub": sub, "aud": "ruto", "azp": "ruto",
		"exp": now.Add(5 * time.Minute).Unix(), "iat": now.Unix(), "auth_time": now.Add(-time.Minute).Unix(),
		"sid": "kc-session", "preferred_username": "alice", "email": "alice@example.test",
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

func (f *fakeSessions) LoginPath(returnPath, prompt string) string {
	query := url.Values{"redirect": {returnPath}}
	if prompt != "" {
		query.Set("prompt", prompt)
	}
	return "/_auth/corp/login?" + query.Encode()
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
		"exp": now.Add(5 * time.Minute).Unix(), "iat": now.Unix(), "sid": "kc-session",
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
