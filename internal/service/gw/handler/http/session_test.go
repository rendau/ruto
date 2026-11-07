package http

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	jwtv5 "github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rendau/ruto/internal/constant"
	appModel "github.com/rendau/ruto/internal/domain/app/model"
	authModel "github.com/rendau/ruto/internal/domain/auth/model"
	endpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
	rootModel "github.com/rendau/ruto/internal/domain/root/model"
	"github.com/rendau/ruto/internal/service/gw/handler/http/proxy"
	"github.com/rendau/ruto/internal/service/gw/service/jwk"
	"github.com/rendau/ruto/internal/service/gw/service/oidc"
)

const sessionTestKid = "session-kid"

// sessionOP is an OpenID provider that only refreshes: "rt-1" and "rt-par"
// are live refresh tokens, anything else is a session that is over.
type sessionOP struct {
	t          *testing.T
	server     *httptest.Server
	privateKey *rsa.PrivateKey
	issuer     string
	calls      atomic.Int32
	delay      time.Duration
}

func newSessionOP(t *testing.T) *sessionOP {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	op := &sessionOP{t: t, privateKey: privateKey}
	mux := http.NewServeMux()
	mux.HandleFunc("/realms/corp/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer":                 op.issuer,
			"authorization_endpoint": op.issuer + "/auth",
			"token_endpoint":         op.issuer + "/token",
			"jwks_uri":               op.issuer + "/certs",
		})
	})
	mux.HandleFunc("/realms/corp/certs", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(jwk.ItemSet{Keys: []*jwk.Item{{
			Kty: "RSA",
			Use: "sig",
			Alg: "RS256",
			Kid: sessionTestKid,
			N:   base64.RawURLEncoding.EncodeToString(privateKey.N.Bytes()),
			E:   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(privateKey.E)).Bytes()),
		}}})
	})
	mux.HandleFunc("/realms/corp/token", func(w http.ResponseWriter, r *http.Request) {
		op.calls.Add(1)
		time.Sleep(op.delay)
		_ = r.ParseForm()
		switch r.PostForm.Get("refresh_token") {
		case "rt-1", "rt-par":
			_ = json.NewEncoder(w).Encode(oidc.TokenResponse{
				AccessToken:      op.token(time.Now().Add(5 * time.Minute)),
				RefreshToken:     "rt-2",
				RefreshExpiresIn: 1800,
			})
		default:
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid_grant"})
		}
	})

	op.server = httptest.NewServer(mux)
	t.Cleanup(op.server.Close)
	op.issuer = op.server.URL + "/realms/corp"
	return op
}

func (op *sessionOP) token(exp time.Time) string {
	tokenObj := jwtv5.NewWithClaims(jwtv5.SigningMethodRS256, jwtv5.MapClaims{"sub": "u1", "iss": op.issuer, "exp": exp.Unix()})
	tokenObj.Header["kid"] = sessionTestKid
	token, err := tokenObj.SignedString(op.privateKey)
	require.NoError(op.t, err)
	return token
}

type backendSeen struct {
	mu            sync.Mutex
	authorization string
	cookies       []string
}

func newSessionTestService(t *testing.T, op *sessionOP, transparent bool) (*Service, *backendSeen) {
	seenByBackend := &backendSeen{}
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenByBackend.mu.Lock()
		seenByBackend.authorization = r.Header.Get("Authorization")
		seenByBackend.cookies = r.Header.Values("Cookie")
		seenByBackend.mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(backend.Close)

	provider := rootModel.RootJwt{
		Name:               "corp",
		Issuer:             op.issuer,
		PublicUrl:          "https://api.example.test",
		Client:             &authModel.JwtClient{Id: "ruto", Secret: "s"},
		TransparentRefresh: transparent,
	}
	jwk.Ins().SetProviders([]rootModel.RootJwt{provider})
	t.Cleanup(func() { jwk.Ins().SetProviders(nil) })
	require.Eventually(t, func() bool {
		key, _ := jwk.Ins().GetPublicKey("corp", sessionTestKid)
		return key != nil
	}, 3*time.Second, 20*time.Millisecond)

	snapshot := rootModel.NewEmpty()
	snapshot.Jwt = []rootModel.RootJwt{provider}
	snapshot.Apps = []*appModel.App{{
		Active:     true,
		PathPrefix: "/account",
		Name:       "account",
		Backend:    appModel.Backend{Url: backend.URL},
		Endpoints: []*endpointModel.Endpoint{{
			Active: true,
			Type:   endpointModel.TypeHTTP,
			Http:   endpointModel.Http{Method: http.MethodGet, Path: "profile"},
			Auth: authModel.Auth{
				Enabled: true,
				Mode:    constant.AuthModeReplace,
				Methods: []*authModel.AuthMethod{{JWT: &authModel.AuthMethodJWT{Provider: "corp"}}},
			},
			Cache: endpointModel.Cache{Browser: endpointModel.CacheBrowser{
				Enabled: true, Scope: constant.CacheScopePublic, MaxAge: 60,
			}},
		}},
	}}
	require.NoError(t, snapshot.Normalize())
	snapshot.InheritDown()
	snapshot.Interpolate()

	service, err := New(snapshot, proxy.NewTransport(0), nil)
	require.NoError(t, err)
	return service, seenByBackend
}

func refreshCookieValue(refreshToken string) string {
	raw, _ := json.Marshal(map[string]string{"rt": refreshToken, "idt": "id-token"})
	return base64.RawURLEncoding.EncodeToString(raw)
}

func sessionRequest(service *Service, cookie string) *httptest.ResponseRecorder {
	return performRequest(service, http.MethodGet, "/account/profile", http.Header{"Cookie": {cookie}})
}

func responseCookie(rec *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

func TestTransparentRefresh(t *testing.T) {
	op := newSessionOP(t)
	service, seenByBackend := newSessionTestService(t, op, true)
	expired := op.token(time.Now().Add(-time.Minute))

	// an expired access token with a live session: renewed on the way
	rec := sessionRequest(service, "other=1; ruto_corp="+expired+"; ruto-rt_corp="+refreshCookieValue("rt-1"))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, int32(1), op.calls.Load())

	access := responseCookie(rec, "ruto_corp")
	require.NotNil(t, access, "the new access cookie is set")
	assert.Equal(t, "/", access.Path)
	assert.NotEqual(t, expired, access.Value)
	refresh := responseCookie(rec, "ruto-rt_corp")
	require.NotNil(t, refresh)
	assert.Equal(t, "/", refresh.Path, "with transparent refresh the refresh cookie comes with app requests")
	assert.Equal(t, 1800, refresh.MaxAge)

	assert.Equal(t, "Bearer "+access.Value, seenByBackend.authorization, "the backend gets the new token")
	assert.Equal(t, []string{"other=1; ruto_corp=" + access.Value}, seenByBackend.cookies,
		"the backend gets the new access cookie and never the refresh one")
	assert.Equal(t, "private, max-age=60", rec.Header().Get("Cache-Control"), "a response with the user's cookies is not public")

	// the renewed cookies work without the provider
	rec = sessionRequest(service, "ruto_corp="+access.Value+"; ruto-rt_corp="+refresh.Value)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, rec.Header().Values("Set-Cookie"))
	assert.Equal(t, "public, max-age=60", rec.Header().Get("Cache-Control"))
	assert.Equal(t, int32(1), op.calls.Load())

	// the session is over: 401 as without the renewal, cookies untouched
	rec = sessionRequest(service, "ruto_corp="+expired+"; ruto-rt_corp="+refreshCookieValue("rt-dead"))
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Empty(t, rec.Header().Values("Set-Cookie"))
	assert.Equal(t, int32(2), op.calls.Load())

	// no refresh cookie: nothing to renew, the provider is not asked
	rec = sessionRequest(service, "ruto_corp="+expired)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Equal(t, int32(2), op.calls.Load())

	// no access cookie at all, but the session is there: renewed too
	rec = sessionRequest(service, "ruto-rt_corp="+refreshCookieValue("rt-1"))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.NotNil(t, responseCookie(rec, "ruto_corp"))

	// an expired bearer from the client wins over the cookies and is not renewed
	calls := op.calls.Load()
	rec = performRequest(service, http.MethodGet, "/account/profile", http.Header{
		"Authorization": {"Bearer " + expired},
		"Cookie":        {"ruto-rt_corp=" + refreshCookieValue("rt-par")},
	})
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Equal(t, calls, op.calls.Load())
}

func TestTransparentRefreshParallel(t *testing.T) {
	op := newSessionOP(t)
	op.delay = 100 * time.Millisecond
	service, _ := newSessionTestService(t, op, true)
	cookie := "ruto_corp=" + op.token(time.Now().Add(-time.Minute)) + "; ruto-rt_corp=" + refreshCookieValue("rt-par")

	// a tab sends ten requests with an expired token at once
	const requests = 10
	codes := make([]int, requests)
	var wg sync.WaitGroup
	for i := range requests {
		wg.Go(func() { codes[i] = sessionRequest(service, cookie).Code })
	}
	wg.Wait()

	for i := range requests {
		assert.Equal(t, http.StatusOK, codes[i], "request %d", i)
	}
	assert.Equal(t, int32(1), op.calls.Load(), "one trip to the provider")
}

func TestNoTransparentRefresh(t *testing.T) {
	op := newSessionOP(t)
	service, _ := newSessionTestService(t, op, false)

	rec := sessionRequest(service, "ruto_corp="+op.token(time.Now().Add(-time.Minute))+"; ruto-rt_corp="+refreshCookieValue("rt-1"))
	assert.Equal(t, http.StatusUnauthorized, rec.Code, "without the flag the front refreshes itself")
	assert.Equal(t, int32(0), op.calls.Load())
}
