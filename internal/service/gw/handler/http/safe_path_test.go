package http

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/rendau/ruto/internal/constant"
	appModel "github.com/rendau/ruto/internal/domain/app/model"
	authModel "github.com/rendau/ruto/internal/domain/auth/model"
	endpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
	rootModel "github.com/rendau/ruto/internal/domain/root/model"
	"github.com/rendau/ruto/internal/service/gw/handler/http/proxy"
)

// A public endpoint must not lead past the auth of a protected one through
// dot segments that the backend would resolve.
func TestService_DotSegmentsRefused(t *testing.T) {
	var backendPaths []string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		backendPaths = append(backendPaths, r.URL.EscapedPath())
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	basic := authModel.Auth{
		Enabled: true,
		Mode:    constant.AuthModeReplace,
		Methods: []*authModel.AuthMethod{{
			Basic: &authModel.AuthMethodBasic{
				Users: []authModel.AuthMethodBasicUser{{Username: "u", Password: "p"}},
			},
		}},
	}

	snapshot := rootModel.NewEmpty()
	snapshot.Apps = []*appModel.App{{
		Active:        true,
		PathPrefix:    "/app",
		Name:          "app",
		AllowWildcard: true,
		Backend:       appModel.Backend{Url: backend.URL},
		Endpoints: []*endpointModel.Endpoint{
			{Id: "1", Active: true, Http: endpointModel.Http{Method: http.MethodGet, Path: "admin"}, Auth: basic},
			{Id: "2", Active: true, Http: endpointModel.Http{Method: http.MethodGet, Path: "public/*"}},
			{Id: "3", Active: true, Http: endpointModel.Http{Method: http.MethodGet, Path: "users/{id}/info"}},
		},
	}}
	require.NoError(t, snapshot.Normalize())
	snapshot.InheritDown()
	snapshot.Interpolate()

	service, err := New(snapshot, proxy.NewTransport(0), nil, nil, nil, nil, nil)
	require.NoError(t, err)

	refused := []string{
		"/app/public/../admin",
		"/app/public/..%2fadmin",
		"/app/public/..%2Fadmin",
		"/app/public/%2e%2e/admin",
		"/app/public/%2E%2E/admin",
		"/app/public/..%5cadmin",
		"/app/public/..;/admin",
		"/app/public/./x",
		"/app/users/..%2fadmin%3f/info",
	}
	for _, p := range refused {
		rec := performRequest(service, http.MethodGet, p, nil)
		require.Equal(t, http.StatusBadRequest, rec.Code, p)
	}
	require.Empty(t, backendPaths)

	allowed := []string{
		"/app/public/x",
		"/app/public/a%2Fb",
		"/app/public/..x",
		"/app/public/x..",
		"/app/public/...",
		"/app/users/42/info",
	}
	for _, p := range allowed {
		rec := performRequest(service, http.MethodGet, p, nil)
		require.Equal(t, http.StatusOK, rec.Code, p)
	}
	require.Len(t, backendPaths, len(allowed))
}

// Protected concrete routes sitting next to a public "*" or "{param}" route
// must not be bypassed through percent-encoding, repeated slashes, case
// differences, path parameters (";") or backslashes.
func TestService_PathBypassBlocked(t *testing.T) {
	var backendPaths []string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		backendPaths = append(backendPaths, r.URL.Path)
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	basic := authModel.Auth{
		Enabled: true,
		Mode:    constant.AuthModeReplace,
		Methods: []*authModel.AuthMethod{{
			Basic: &authModel.AuthMethodBasic{
				Users: []authModel.AuthMethodBasicUser{{Username: "u", Password: "p"}},
			},
		}},
	}

	snapshot := rootModel.NewEmpty()
	snapshot.Apps = []*appModel.App{{
		Active:        true,
		PathPrefix:    "/shop",
		Name:          "shop",
		AllowWildcard: true,
		Backend:       appModel.Backend{Url: backend.URL},
		Endpoints: []*endpointModel.Endpoint{
			{Id: "1", Active: true, Http: endpointModel.Http{Method: http.MethodGet, Path: "*"}},
			{Id: "2", Active: true, AllowTrailingSlash: true, Http: endpointModel.Http{Method: http.MethodGet, Path: "admin"}, Auth: basic},
			{Id: "3", Active: true, Http: endpointModel.Http{Method: http.MethodGet, Path: "admin/secret"}, Auth: basic},
			{Id: "4", Active: true, Http: endpointModel.Http{Method: http.MethodGet, Path: "users/{id}"}},
			{Id: "5", Active: true, Http: endpointModel.Http{Method: http.MethodGet, Path: "users/me"}, Auth: basic},
			{Id: "6", Active: true, Http: endpointModel.Http{Method: http.MethodGet, Path: "users/admin/delete"}, Auth: basic},
			{Id: "7", Active: true, Http: endpointModel.Http{Method: http.MethodGet, Path: "items/{code:[A-Z0-9]{2,4}}"}},
		},
	}}
	require.NoError(t, snapshot.Normalize())
	snapshot.InheritDown()
	snapshot.Interpolate()

	service, err := New(snapshot, proxy.NewTransport(0), nil, nil, nil, nil, nil)
	require.NoError(t, err)

	blocked := []struct {
		path string
		code int
	}{
		// Percent-encoding against "*" and "{id}"
		{path: "/shop/%61dmin", code: http.StatusUnauthorized},
		{path: "/shop/%41dmin", code: http.StatusUnauthorized},
		{path: "/shop/%61dmin/", code: http.StatusUnauthorized},
		{path: "/shop/users/m%65", code: http.StatusUnauthorized},
		{path: "/shop/admin%2fsecret", code: http.StatusUnauthorized},
		{path: "/shop/admin%2Fsecret", code: http.StatusUnauthorized},
		{path: "/shop/users/admin%2fdelete", code: http.StatusUnauthorized},
		{path: "/%73hop/admin", code: http.StatusUnauthorized},
		// Case variation against "*" and "{id}"
		{path: "/shop/ADMIN", code: http.StatusUnauthorized},
		{path: "/shop/Admin", code: http.StatusUnauthorized},
		{path: "/shop/ADMIN/", code: http.StatusUnauthorized},
		{path: "/shop/ADMIN/SECRET", code: http.StatusUnauthorized},
		{path: "/shop/users/ME", code: http.StatusUnauthorized},
		{path: "/shop/users/Me", code: http.StatusUnauthorized},
		{path: "/shop/USERS/me", code: http.StatusUnauthorized},
		{path: "/SHOP/admin", code: http.StatusUnauthorized},
		// Repeated slashes
		{path: "/shop//admin", code: http.StatusBadRequest},
		{path: "//shop/admin", code: http.StatusBadRequest},
		{path: "/shop/%2fadmin", code: http.StatusBadRequest},
		{path: "/shop/admin//", code: http.StatusBadRequest},
		{path: "/shop/admin//secret", code: http.StatusBadRequest},
		{path: "/shop/users//me", code: http.StatusBadRequest},
		// Semicolon / matrix path parameters
		{path: "/shop/admin;x", code: http.StatusBadRequest},
		{path: "/shop/admin;jsessionid=1", code: http.StatusBadRequest},
		{path: "/shop/admin%3bx", code: http.StatusBadRequest},
		{path: "/shop/admin;x/secret", code: http.StatusBadRequest},
		{path: "/shop/users/me;x", code: http.StatusBadRequest},
		{path: "/shop/users/me%3Bx", code: http.StatusBadRequest},
		// Backslashes and control characters
		{path: "/shop/admin%5csecret", code: http.StatusBadRequest},
		{path: "/shop/admin%00", code: http.StatusBadRequest},
		{path: "/shop/admin%0a", code: http.StatusBadRequest},
		{path: "/shop/users/me%00", code: http.StatusBadRequest},
	}

	// Verify both through direct ServeHTTP and through a real net/http server
	// so wire-level request-target parsing is covered too.
	gwServer := httptest.NewServer(service)
	defer gwServer.Close()
	gwURL, err := url.Parse(gwServer.URL)
	require.NoError(t, err)

	for _, tc := range blocked {
		rec := performRequest(service, http.MethodGet, tc.path, nil)
		require.Equal(t, tc.code, rec.Code, "direct %s", tc.path)

		statusCode := sendRawHTTPRequest(t, gwURL.Host, tc.path)
		require.Equal(t, tc.code, statusCode, "wire %s", tc.path)
	}
	require.Empty(t, backendPaths, "no blocked request may reach the backend")

	// Legitimate requests still work and keep their original case for the backend.
	allowed := []struct {
		path            string
		headers         http.Header
		wantBackendPath string
	}{
		{path: "/shop/users/JohnDoe", wantBackendPath: "/users/JohnDoe"},
		{path: "/shop/products/%D0%BA%D0%BE%D1%84%D0%B5", wantBackendPath: "/products/кофе"},
		{path: "/shop/products/%d0%ba%d0%be%d1%84%d0%b5", wantBackendPath: "/products/кофе"},
		{path: "/shop/files/my%20file.txt", wantBackendPath: "/files/my file.txt"},
		{path: "/shop/items/AB12", wantBackendPath: "/items/AB12"},
		{
			path:            "/shop/admin",
			headers:         http.Header{"Authorization": {"Basic " + base64.StdEncoding.EncodeToString([]byte("u:p"))}},
			wantBackendPath: "/admin",
		},
		{
			path:            "/shop/ADMIN",
			headers:         http.Header{"Authorization": {"Basic " + base64.StdEncoding.EncodeToString([]byte("u:p"))}},
			wantBackendPath: "/ADMIN",
		},
	}
	for _, tc := range allowed {
		backendPaths = nil
		rec := performRequest(service, http.MethodGet, tc.path, tc.headers)
		require.Equal(t, http.StatusOK, rec.Code, tc.path)
		require.Equal(t, []string{tc.wantBackendPath}, backendPaths, tc.path)
	}
}

func TestLowerRoutePattern(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "/Shop/Admin", want: "/shop/admin"},
		{in: "/Shop/Users/{userId}/Profile", want: "/shop/users/{userId}/profile"},
		{in: "/Shop/Docs/*", want: "/shop/docs/*"},
		{in: "/Shop/Items/{code:[A-Z0-9]{2,4}}", want: "/shop/items/{code:(?i)[A-Z0-9]{2,4}}"},
		{in: `/Shop/Items/{code:\D+}`, want: `/shop/items/{code:(?i)\D+}`},
		{in: "/Shop/Items/{code:^[A-Z]+$}", want: "/shop/items/{code:^(?i)[A-Z]+$}"},
		{in: "/Shop/Items/{a:[A-Z]+}-{b:[A-Z]+}", want: "/shop/items/{a:(?i)[A-Z]+}-{b:(?i)[A-Z]+}"},
	}

	for _, tc := range tests {
		require.Equal(t, tc.want, lowerRoutePattern(tc.in), tc.in)
	}
}

// sendRawHTTPRequest writes a raw HTTP/1.1 request line so net/http.Client
// does not clean or re-encode the path before it reaches the server.
func sendRawHTTPRequest(t *testing.T, addr, rawPath string) int {
	t.Helper()

	conn, err := net.Dial("tcp", addr)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	_, err = fmt.Fprintf(conn, "GET %s HTTP/1.1\r\nHost: %s\r\nConnection: close\r\n\r\n", rawPath, addr)
	require.NoError(t, err)

	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	require.NoError(t, err)
	_ = resp.Body.Close()

	return resp.StatusCode
}
