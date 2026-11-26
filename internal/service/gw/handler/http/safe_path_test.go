package http

import (
	"net/http"
	"net/http/httptest"
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
