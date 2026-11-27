package core

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/rendau/ruto/internal/errs"
	"github.com/rendau/ruto/internal/infra/clientip"
)

type fakeAdminLogin struct {
	clientIP netip.Addr
}

func (f *fakeAdminLogin) Login(_ context.Context, username, password string, clientIP netip.Addr) (string, error) {
	f.clientIP = clientIP
	switch {
	case username == "admin" && password == "pass":
		return "jwt-token", nil
	case username == "locked":
		return "", errs.TooManyAttempts
	}
	return "", errs.NotAuthorized
}

func sessionRequest(method, origin, body string) *http.Request {
	req := httptest.NewRequest(method, "http://ruto.example.com/api/session", strings.NewReader(body))
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	return req
}

func TestAdminSessionLogin(t *testing.T) {
	login := &fakeAdminLogin{}
	handler := NewAdminSessionHandler(login, clientip.New(nil, nil))

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, sessionRequest(http.MethodPost, "https://ruto.example.com", `{"username":"admin","password":"pass"}`))
	require.Equal(t, http.StatusNoContent, rec.Code)
	require.Equal(t, netip.MustParseAddr("192.0.2.1"), login.clientIP, "the throttle gets the client address")
	require.Empty(t, rec.Body.String(), "the token must not reach the page")

	cookies := rec.Result().Cookies()
	require.Len(t, cookies, 1)
	c := cookies[0]
	require.Equal(t, adminSessionCookie, c.Name)
	require.Equal(t, "jwt-token", c.Value)
	require.Equal(t, "/api", c.Path)
	require.True(t, c.HttpOnly)
	require.True(t, c.Secure)
	require.Equal(t, http.SameSiteStrictMode, c.SameSite)
	require.Equal(t, 12*60*60, c.MaxAge)

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, sessionRequest(http.MethodPost, "https://ruto.example.com", `{"username":"admin","password":"wrong"}`))
	require.Equal(t, http.StatusUnauthorized, rec.Code)
	require.Contains(t, rec.Body.String(), `"code":"not_authorized"`)
	require.Empty(t, rec.Result().Cookies())

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, sessionRequest(http.MethodPost, "https://ruto.example.com", `{"username":"locked","password":"pass"}`))
	require.Equal(t, http.StatusTooManyRequests, rec.Code)
	require.Contains(t, rec.Body.String(), `"code":"too_many_attempts"`)
	require.Empty(t, rec.Result().Cookies())

	for _, origin := range []string{"", "https://evil.example.com", "https://other.example.com"} {
		rec = httptest.NewRecorder()
		handler.ServeHTTP(rec, sessionRequest(http.MethodPost, origin, `{"username":"admin","password":"pass"}`))
		require.Equal(t, http.StatusForbidden, rec.Code, origin)
		require.Empty(t, rec.Result().Cookies(), origin)
	}
}

func TestAdminSessionLogout(t *testing.T) {
	rec := httptest.NewRecorder()
	NewAdminSessionHandler(&fakeAdminLogin{}, clientip.New(nil, nil)).ServeHTTP(rec, sessionRequest(http.MethodDelete, "https://ruto.example.com", ""))
	require.Equal(t, http.StatusNoContent, rec.Code)

	cookies := rec.Result().Cookies()
	require.Len(t, cookies, 1)
	require.Equal(t, adminSessionCookie, cookies[0].Name)
	require.Empty(t, cookies[0].Value)
	require.Less(t, cookies[0].MaxAge, 0)
}

func TestSessionCSRFGuard(t *testing.T) {
	handler := NewSessionCSRFGuard(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	tests := []struct {
		name   string
		method string
		cookie bool
		bearer bool
		origin string
		want   int
	}{
		{"cookie, same origin", http.MethodPost, true, false, "https://ruto.example.com", http.StatusOK},
		{"cookie, other subdomain", http.MethodPost, true, false, "https://tiny.example.com", http.StatusForbidden},
		{"cookie, no origin", http.MethodDelete, true, false, "", http.StatusForbidden},
		{"cookie, read", http.MethodGet, true, false, "https://tiny.example.com", http.StatusOK},
		{"bearer from another origin", http.MethodPost, false, true, "https://tiny.example.com", http.StatusOK},
		{"cookie and bearer", http.MethodPost, true, true, "https://tiny.example.com", http.StatusOK},
		{"no credentials", http.MethodPost, false, false, "https://tiny.example.com", http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "http://ruto.example.com/api/app", nil)
			if tt.cookie {
				req.AddCookie(&http.Cookie{Name: adminSessionCookie, Value: "jwt-token"})
			}
			if tt.bearer {
				req.Header.Set("Authorization", "Bearer other")
			}
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			require.Equal(t, tt.want, rec.Code)
		})
	}
}

func TestGrpcGatewaySessionMetadata(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://ruto.example.com/api/app", nil)
	require.Nil(t, grpcGatewaySessionMetadata(context.Background(), req))

	req.AddCookie(&http.Cookie{Name: adminSessionCookie, Value: "jwt-token"})
	require.Equal(t, []string{"Bearer jwt-token"}, grpcGatewaySessionMetadata(context.Background(), req).Get("authorization"))

	req.Header.Set("Authorization", "Bearer key")
	require.Nil(t, grpcGatewaySessionMetadata(context.Background(), req))
}
