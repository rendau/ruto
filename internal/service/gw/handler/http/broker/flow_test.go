package broker

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	snapshotModel "github.com/rendau/ruto/internal/domain/snapshot/model"
	"github.com/rendau/ruto/internal/service/gw/handler/http/authflow"
	"github.com/rendau/ruto/internal/service/gw/service/apptoken"
	gwAuthModel "github.com/rendau/ruto/internal/service/gw/service/auth/model"
	"github.com/rendau/ruto/internal/service/gw/service/rutokey"
)

const (
	crmBase      = "/_auth/corp/crm/realms/corp/protocol/openid-connect"
	crmIssuer    = "https://api.test/_auth/corp/crm/realms/corp"
	crmCallback  = "https://crm.front.test/callback"
	testVerifier = "verifier-0123456789-0123456789-0123456789-abc"
)

func challengeOf(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

type flow struct {
	t        *testing.T
	h        http.Handler
	sessions *fakeSessions
	keys     *rutokey.Service
	used     *fakeUsed
}

func newFlow(t *testing.T) *flow {
	t.Helper()
	authflow.SetCookieKey(snapshotModel.CookieKeyFrom("broker flow test"))
	t.Cleanup(func() { authflow.SetCookieKey(nil) })
	f := &flow{t: t, sessions: newFakeSessions(t), keys: newKeys(t), used: newFakeUsed()}
	gwAuthModel.SetRutoKids(f.keys.Kids())
	t.Cleanup(func() { gwAuthModel.SetRutoKids(nil) })
	f.h = newRouterWith(t, Deps{
		Meta: &fakeMeta{docs: map[string]string{providerIssuer: keycloakDoc}},
		Keys: f.keys,
		Used: f.used,
	}, f.sessions)
	return f
}

func authorizeQuery(overrides ...string) url.Values {
	query := url.Values{
		"client_id":             {"crm"},
		"redirect_uri":          {crmCallback},
		"response_type":         {"code"},
		"scope":                 {"openid profile"},
		"state":                 {"st-1"},
		"nonce":                 {"n-1"},
		"code_challenge":        {challengeOf(testVerifier)},
		"code_challenge_method": {"S256"},
	}
	for i := 0; i+1 < len(overrides); i += 2 {
		if overrides[i+1] == "" {
			query.Del(overrides[i])
		} else {
			query.Set(overrides[i], overrides[i+1])
		}
	}
	return query
}

func (f *flow) authorize(query url.Values) *httptest.ResponseRecorder {
	return get(f.t, f.h, crmBase+"/auth?"+query.Encode())
}

// answer parses the redirect to the front: the parameters of the query or
// of the fragment.
func answer(t *testing.T, rec *httptest.ResponseRecorder) url.Values {
	t.Helper()
	require.Equal(t, http.StatusFound, rec.Code, rec.Body.String())
	location, err := url.Parse(rec.Header().Get("Location"))
	require.NoError(t, err)
	require.Equal(t, "crm.front.test", location.Host)
	if location.Fragment != "" {
		values, err := url.ParseQuery(location.Fragment)
		require.NoError(t, err)
		return values
	}
	return location.Query()
}

func (f *flow) code() string {
	values := answer(f.t, f.authorize(authorizeQuery()))
	require.NotEmpty(f.t, values.Get("code"), values)
	return values.Get("code")
}

func (f *flow) exchange(form url.Values, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, crmBase+"/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	f.h.ServeHTTP(rec, req)
	return rec
}

func codeForm(code string, overrides ...string) url.Values {
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"client_id":     {"crm"},
		"redirect_uri":  {crmCallback},
		"code_verifier": {testVerifier},
	}
	for i := 0; i+1 < len(overrides); i += 2 {
		form.Set(overrides[i], overrides[i+1])
	}
	return form
}

func tokenErrorOf(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body), rec.Body.String())
	return body["error"]
}

// verify checks a ruto token by the broker's JWKS, as a front's library does.
func (f *flow) verify(token string) jwt.MapClaims {
	f.t.Helper()
	rec := get(f.t, f.h, crmBase+"/certs")
	require.Equal(f.t, http.StatusOK, rec.Code)
	keys := map[string]any{}
	for kid, key := range publicKeysOf(f.t, rec.Body.Bytes()) {
		keys[kid] = key
	}
	claims := jwt.MapClaims{}
	_, err := jwt.ParseWithClaims(token, claims, func(tok *jwt.Token) (any, error) {
		return keys[tok.Header["kid"].(string)], nil
	}, jwt.WithIssuer(crmIssuer), jwt.WithAudience("crm"), jwt.WithExpirationRequired(), jwt.WithValidMethods([]string{"ES256"}))
	require.NoError(f.t, err)
	return claims
}

func TestAuthorizeWithSession(t *testing.T) {
	f := newFlow(t)

	values := answer(t, f.authorize(authorizeQuery()))
	assert.NotEmpty(t, values.Get("code"))
	assert.Equal(t, "st-1", values.Get("state"))
	assert.Equal(t, crmIssuer, values.Get("iss"), "RFC 9207")

	values = answer(t, f.authorize(authorizeQuery("response_mode", "fragment")))
	assert.NotEmpty(t, values.Get("code"), "keycloak-js asks for the fragment")
	assert.Equal(t, "st-1", values.Get("state"))
}

func TestAuthorizeWithoutSession(t *testing.T) {
	f := newFlow(t)
	f.sessions.logout(authflow.ErrNoSession)

	rec := f.authorize(authorizeQuery("prompt", "select_account"))
	require.Equal(t, http.StatusFound, rec.Code)
	login, err := url.Parse(rec.Header().Get("Location"))
	require.NoError(t, err)
	assert.Equal(t, "/_auth/corp/login", login.Path, "the gateway's own login at the provider")
	assert.Equal(t, "select_account", login.Query().Get("prompt"), "passed to the provider")
	back, err := url.Parse(login.Query().Get("redirect"))
	require.NoError(t, err)
	assert.Equal(t, crmBase+"/auth", back.Path, "comes back to the same request")
	assert.Empty(t, back.Query().Get("prompt"), "done with at the provider")
	assert.Equal(t, "st-1", back.Query().Get("state"))
	assert.Equal(t, challengeOf(testVerifier), back.Query().Get("code_challenge"))

	// the login failed at the provider and came back with auth_error
	values := answer(t, get(t, f.h, back.String()+"&auth_error=access_denied"))
	assert.Equal(t, "access_denied", values.Get("error"))
	assert.Equal(t, "st-1", values.Get("state"))

	values = answer(t, f.authorize(authorizeQuery("prompt", "none")))
	assert.Equal(t, "login_required", values.Get("error"), "no interaction asked, none possible")

	f.sessions.logout(authflow.ErrUnavailable)
	values = answer(t, f.authorize(authorizeQuery()))
	assert.Equal(t, "temporarily_unavailable", values.Get("error"))
}

func TestAuthorizePromptLogin(t *testing.T) {
	f := newFlow(t)

	rec := f.authorize(authorizeQuery("prompt", "login"))
	require.Equal(t, http.StatusFound, rec.Code)
	login, err := url.Parse(rec.Header().Get("Location"))
	require.NoError(t, err)
	assert.Equal(t, "/_auth/corp/login", login.Path, "a live session does not count, the person logs in again")
	assert.Equal(t, "login", login.Query().Get("prompt"))
	assert.NotContains(t, login.Query().Get("redirect"), "prompt", "no loop after the login")
}

func TestAuthorizeChecks(t *testing.T) {
	f := newFlow(t)

	// nowhere to answer: shown here
	for name, query := range map[string]url.Values{
		"another client":         authorizeQuery("client_id", "cart-fe"),
		"no redirect_uri":        authorizeQuery("redirect_uri", ""),
		"foreign redirect_uri":   authorizeQuery("redirect_uri", "https://evil.test/cb"),
		"wildcard does not let":  authorizeQuery("redirect_uri", "https://other.test/cb"),
		"relative redirect_uri":  authorizeQuery("redirect_uri", "/callback"),
		"redirect with fragment": authorizeQuery("redirect_uri", crmCallback+"#x"),
	} {
		rec := f.authorize(query)
		assert.Equal(t, http.StatusBadRequest, rec.Code, name)
		assert.Empty(t, rec.Header().Get("Location"), name)
	}

	// answered at the front's address
	for want, query := range map[string]url.Values{
		"invalid_request":           authorizeQuery("code_challenge", ""),
		"unsupported_response_type": authorizeQuery("response_type", "token"),
	} {
		assert.Equal(t, want, answer(t, f.authorize(query)).Get("error"))
	}
	assert.Equal(t, "invalid_request", answer(t, f.authorize(authorizeQuery("code_challenge_method", "plain"))).Get("error"))
	assert.Equal(t, "invalid_request", answer(t, f.authorize(authorizeQuery("response_mode", "form_post"))).Get("error"))
	assert.Equal(t, "invalid_request", answer(t, f.authorize(authorizeQuery("prompt", "none login"))).Get("error"))
	assert.Equal(t, "server_error", answer(t, f.authorize(authorizeQuery("auth_error", "<script>"))).Get("error"))

	// the gateway's own origin is a return address too
	rec := f.authorize(authorizeQuery("redirect_uri", "https://api.test/app/callback"))
	require.Equal(t, http.StatusFound, rec.Code)
	assert.True(t, strings.HasPrefix(rec.Header().Get("Location"), "https://api.test/app/callback?"))
}

func TestCodeGrant(t *testing.T) {
	f := newFlow(t)

	rec := f.exchange(codeForm(f.code()))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
	var rep tokenResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &rep))
	assert.Equal(t, "Bearer", rep.TokenType)
	assert.InDelta(t, 300, rep.ExpiresIn, 2, "as long as the shared token")
	assert.Equal(t, "openid profile", rep.Scope)

	id := f.verify(rep.IdToken)
	assert.Equal(t, "u-1", id["sub"])
	assert.Equal(t, "crm", id["azp"])
	assert.Equal(t, "n-1", id["nonce"])
	assert.EqualValues(t, f.sessions.session.AuthTime, id["auth_time"])
	assert.Equal(t, "alice", id["preferred_username"], "the person's data from the front token")
	assert.Equal(t, map[string]any{"crm": map[string]any{"roles": []any{"admin"}}}, id["resource_access"], "the front's roles")
	assert.NotContains(t, id, "sid", "service claims of the provider stay out")

	access := f.verify(rep.AccessToken)
	assert.Equal(t, "Bearer", access["typ"])
	assert.Equal(t, "openid profile", access["scope"])
	assert.Equal(t, id["resource_access"], access["resource_access"], "keycloak-js reads roles from the access token")
	assert.NotContains(t, access, "nonce")

	refresh := f.verify(rep.RefreshToken)
	assert.Equal(t, "Refresh", refresh["typ"])
	assert.InDelta(t, f.sessions.session.SessionExpires.Unix(), refresh["exp"], 1, "as long as the session")

	kid, _, err := jwt.NewParser().ParseUnverified(rep.AccessToken, jwt.MapClaims{})
	require.NoError(t, err)
	assert.Equal(t, f.keys.Kids()[0], kid.Header["kid"], "the gateway knows its own tokens")

	// one exchange for the front token and the backend with the same audience
	assert.Equal(t, int32(1), f.sessions.exchanges.Load())
	front := cookieOf(t, rec, "ruto-ft_corp_crm")
	assert.Equal(t, "/_auth/corp/crm", front.Path)
	assert.InDelta(t, 300, front.MaxAge, 2)
	assert.True(t, front.HttpOnly)
	backend := cookieOf(t, rec, "ruto_corp_crm-api")
	assert.Equal(t, "/crm-api", backend.Path)
	assert.Equal(t, front.Value, backend.Value)
	assert.Nil(t, findCookie(rec, "ruto_corp_orders"), "another audience is not touched")

	// the next login of the front takes the front token from its cookie
	rec = f.exchange(codeForm(f.code()), &http.Cookie{Name: front.Name, Value: front.Value})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, int32(1), f.sessions.exchanges.Load(), "no new exchange")
}

func TestCodeGrantChecks(t *testing.T) {
	f := newFlow(t)

	code := f.code()
	for name, form := range map[string]url.Values{
		"wrong verifier":       codeForm(code, "code_verifier", "another-verifier-0123456789-0123456789-01234"),
		"wrong redirect_uri":   codeForm(code, "redirect_uri", "https://crm.front.test/other"),
		"not a code":           codeForm("garbage"),
		"code of another time": codeForm(f.code()[:20]),
	} {
		rec := f.exchange(form)
		assert.Equal(t, http.StatusBadRequest, rec.Code, name)
		assert.Equal(t, "invalid_grant", tokenErrorOf(t, rec), name)
	}

	rec := f.exchange(codeForm(code, "client_id", "cart-fe"))
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Equal(t, "invalid_client", tokenErrorOf(t, rec))

	// the checks above did not use the code up
	require.Equal(t, http.StatusOK, f.exchange(codeForm(code)).Code)
	rec = f.exchange(codeForm(code))
	assert.Equal(t, "invalid_grant", tokenErrorOf(t, rec), "a code works once")

	// a code of one front does not open for another
	other := httptest.NewRequest(http.MethodPost, "/_auth/corp/cart-fe/realms/corp/protocol/openid-connect/token",
		strings.NewReader(codeForm(f.code(), "client_id", "cart-fe").Encode()))
	other.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	otherRec := httptest.NewRecorder()
	f.h.ServeHTTP(otherRec, other)
	assert.Equal(t, "invalid_grant", tokenErrorOf(t, otherRec))

	// the session changed hands between the code and its exchange
	code = f.code()
	f.sessions.login("u-2")
	assert.Equal(t, "invalid_grant", tokenErrorOf(t, f.exchange(codeForm(code))))

	code = f.code()
	f.sessions.logout(authflow.ErrNoSession)
	assert.Equal(t, "invalid_grant", tokenErrorOf(t, f.exchange(codeForm(code))))

	rec = f.exchange(url.Values{"grant_type": {"password"}, "username": {"alice"}, "client_id": {"crm"}})
	assert.Equal(t, "unsupported_grant_type", tokenErrorOf(t, rec))
	assert.Equal(t, http.StatusMethodNotAllowed, get(t, f.h, crmBase+"/token").Code)
}

func TestCodeExpires(t *testing.T) {
	f := newFlow(t)
	u := &unit{name: "corp"}
	value, ok := u.issueCode("crm", &loginCode{RedirectUri: crmCallback, Challenge: challengeOf(testVerifier), Sub: "u-1"})
	require.True(t, ok)
	_, ok = u.openCode("crm", value)
	assert.True(t, ok)

	raw, ok := authflow.Open(u.codeSealName("crm"), value)
	require.True(t, ok)
	var code loginCode
	require.NoError(t, json.Unmarshal(raw, &code))
	code.Expires = time.Now().Add(-time.Second).Unix()
	raw, _ = json.Marshal(code)
	expired, _ := authflow.Seal(u.codeSealName("crm"), raw)
	assert.Equal(t, "invalid_grant", tokenErrorOf(t, f.exchange(codeForm(expired))))
}

func TestCodeGrantExchangeRefusals(t *testing.T) {
	f := newFlow(t)

	// the person has nothing in this front: logs in on the shared token's data
	f.sessions.exchangeErr = apptoken.ErrRefused
	rec := f.exchange(codeForm(f.code()))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var rep tokenResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &rep))
	id := f.verify(rep.IdToken)
	assert.Equal(t, "alice", id["preferred_username"])
	assert.NotContains(t, id, "resource_access")
	assert.Nil(t, findCookie(rec, "ruto-ft_corp_crm"))

	f.sessions.exchangeErr = apptoken.ErrTokenRejected
	assert.Equal(t, "invalid_grant", tokenErrorOf(t, f.exchange(codeForm(f.code()))))

	f.sessions.exchangeErr = apptoken.ErrUnavailable
	rec = f.exchange(codeForm(f.code()))
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.Equal(t, "temporarily_unavailable", tokenErrorOf(t, rec))
}

func findCookie(rec *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

func cookieOf(t *testing.T, rec *httptest.ResponseRecorder, name string) *http.Cookie {
	t.Helper()
	c := findCookie(rec, name)
	require.NotNil(t, c, "cookie %s", name)
	return c
}
