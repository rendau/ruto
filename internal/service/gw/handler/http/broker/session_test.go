package broker

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rendau/ruto/internal/service/gw/handler/http/authflow"
)

// login runs a front login through the broker and returns ruto's tokens and
// the front token cookie it left.
func (f *flow) login() (*tokenResponse, *http.Cookie) {
	f.t.Helper()
	rec := f.exchange(codeForm(f.code()))
	require.Equal(f.t, http.StatusOK, rec.Code, rec.Body.String())
	var rep tokenResponse
	require.NoError(f.t, json.Unmarshal(rec.Body.Bytes(), &rep))
	front := cookieOf(f.t, rec, "ruto-ft_corp_crm")
	return &rep, &http.Cookie{Name: front.Name, Value: front.Value}
}

func (f *flow) refresh(refreshToken string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	return f.exchange(url.Values{"grant_type": {"refresh_token"}, "refresh_token": {refreshToken}, "client_id": {"crm"}}, cookies...)
}

func (f *flow) userinfo(token string, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, crmBase+"/userinfo", nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	f.h.ServeHTTP(rec, req)
	return rec
}

func TestRefreshGrant(t *testing.T) {
	f := newFlow(t)
	rep, front := f.login()
	first := f.verify(rep.IdToken)

	rec := f.refresh(rep.RefreshToken, front)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, int32(1), f.sessions.renewals.Load(), "the shared token is renewed for the front")
	assert.Equal(t, int32(1), f.sessions.exchanges.Load(), "the live front token from its cookie serves")
	var renewed tokenResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &renewed))
	assert.Equal(t, "openid profile", renewed.Scope, "kept from the refresh token")

	id := f.verify(renewed.IdToken)
	assert.Equal(t, first["sub"], id["sub"])
	assert.Equal(t, first["azp"], id["azp"])
	assert.Equal(t, first["auth_time"], id["auth_time"], "oidc-client-ts checks it did not change")
	assert.NotContains(t, id, "nonce", "no login, no nonce")
	assert.Equal(t, "openid profile", f.verify(renewed.AccessToken)["scope"])

	// without the front token cookie (it expired): a new exchange
	rec = f.refresh(renewed.RefreshToken)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, int32(2), f.sessions.exchanges.Load())

	// refused
	assert.Equal(t, "invalid_grant", tokenErrorOf(t, f.refresh("garbage")))
	other, _ := f.keys.Sign(jwt.MapClaims{"iss": "https://api.test/_auth/corp/cart-fe/realms/corp", "sub": "u-1"})
	assert.Equal(t, "invalid_grant", tokenErrorOf(t, f.refresh(other)), "a token of another front")

	f.sessions.login("u-2")
	assert.Equal(t, "invalid_grant", tokenErrorOf(t, f.refresh(renewed.RefreshToken)), "the person changed in another tab")

	f.sessions.logout(authflow.ErrNoSession)
	assert.Equal(t, "invalid_grant", tokenErrorOf(t, f.refresh(renewed.RefreshToken)), "the session is over: log in")
}

func TestUserinfoWithToken(t *testing.T) {
	f := newFlow(t)
	rep, front := f.login()

	rec := f.userinfo(rep.AccessToken, front)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "crm", body["aud_of_token"], "asked with the front token")
	assert.Equal(t, map[string]any{"crm": map[string]any{"roles": []any{"admin"}}}, body["resource_access"], "the front's roles")
	assert.NotContains(t, body, "tokens", "a front with tokens gets the provider's answer as it is")
	assert.Equal(t, []string{"http://kc.test/realms/corp/protocol/openid-connect/userinfo"}, f.sessions.userinfoCalls)
	assert.Equal(t, int32(1), f.sessions.exchanges.Load())

	for name, token := range map[string]string{
		"a provider token":   f.sessions.session.AccessToken,
		"not a token at all": "garbage",
	} {
		rec = f.userinfo(token, front)
		assert.Equal(t, http.StatusUnauthorized, rec.Code, name)
		assert.Equal(t, `Bearer error="invalid_token"`, rec.Header().Get("WWW-Authenticate"), name)
	}

	f.sessions.login("u-2")
	assert.Equal(t, http.StatusUnauthorized, f.userinfo(rep.AccessToken, front).Code, "the session is someone else's")

	f.sessions.logout(authflow.ErrNoSession)
	rec = f.userinfo(rep.AccessToken, front)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Equal(t, `Bearer error="invalid_token"`, rec.Header().Get("WWW-Authenticate"))
}

func TestUserinfoWithoutToken(t *testing.T) {
	f := newFlow(t)

	rec := f.userinfo("")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
	var body struct {
		Sub            string         `json:"sub"`
		ResourceAccess map[string]any `json:"resource_access"`
		Tokens         map[string]any `json:"tokens"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "u-1", body.Sub)
	assert.Contains(t, body.ResourceAccess, "crm")
	require.NotNil(t, body.Tokens, "the page opens without a redirect")
	assert.Equal(t, "Bearer", body.Tokens["token_type"])
	assert.InDelta(t, 300, body.Tokens["expires_in"], 2)
	assert.NotContains(t, body.Tokens, "id_token", "no login, no ID token")
	access := f.verify(body.Tokens["access_token"].(string))
	assert.Equal(t, "u-1", access["sub"])
	assert.Equal(t, "Refresh", f.verify(body.Tokens["refresh_token"].(string))["typ"])
	assert.NotNil(t, cookieOf(t, rec, "ruto-ft_corp_crm"), "the front token is got and kept")

	f.sessions.logout(authflow.ErrNoSession)
	rec = f.userinfo("")
	assert.Equal(t, http.StatusUnauthorized, rec.Code, "no session: the SDK logs in as usual")
	assert.Equal(t, "Bearer", rec.Header().Get("WWW-Authenticate"))
}

func TestEndSession(t *testing.T) {
	f := newFlow(t)
	logout := crmBase + "/logout"

	rec := get(t, f.h, logout+"?"+url.Values{
		"client_id": {"crm"}, "post_logout_redirect_uri": {crmCallback}, "state": {"bye"}, "id_token_hint": {"whatever"},
	}.Encode())
	require.Equal(t, http.StatusFound, rec.Code, rec.Body.String())
	assert.Equal(t, crmCallback, rec.Header().Get("Location"))
	assert.Equal(t, []string{crmCallback + "|bye"}, f.sessions.loggedOut, "the gateway's logout, through the provider")
	front := cookieOf(t, rec, "ruto-ft_corp_crm")
	assert.Equal(t, -1, front.MaxAge, "this front's token is dropped")
	assert.Equal(t, "/_auth/corp/crm", front.Path)

	for name, query := range map[string]url.Values{
		"foreign return address": {"post_logout_redirect_uri": {"https://evil.test/"}},
		"another client":         {"client_id": {"cart-fe"}, "post_logout_redirect_uri": {crmCallback}},
	} {
		rec = get(t, f.h, logout+"?"+query.Encode())
		assert.Equal(t, http.StatusBadRequest, rec.Code, name)
		assert.Empty(t, rec.Header().Get("Location"), name)
	}

	rec = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, logout, strings.NewReader("client_id=crm"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	f.h.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusFound, rec.Code)
	assert.Equal(t, "https://api.test/", rec.Header().Get("Location"), "no return address: the gateway root")
}
