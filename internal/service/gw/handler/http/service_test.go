package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	appModel "github.com/rendau/ruto/internal/domain/app/model"
	endpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
	rootModel "github.com/rendau/ruto/internal/domain/root/model"
)

func TestServiceBuild_ProxyByConfig(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Backend-Method", r.Method)
		w.Header().Set("X-Backend-Path", r.URL.Path)
		w.Header().Set("X-Backend-Query", r.URL.RawQuery)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer backend.Close()

	tests := []struct {
		name                 string
		appBackendPathPrefix string
		endpoint             *endpointModel.Endpoint
		requestURL           string
		wantStatus           int
		wantMethod           string
		wantPath             string
		wantRawQuery         string
		checkBackend         bool
	}{
		{
			name:                 "get with query",
			appBackendPathPrefix: "/svc",
			endpoint: &endpointModel.Endpoint{
				Method: http.MethodGet,
				Path:   "users",
			},
			requestURL:   "https://public.example/api/users?q=1",
			wantStatus:   http.StatusNoContent,
			wantMethod:   http.MethodGet,
			wantPath:     "/svc/users",
			wantRawQuery: "q=1",
			checkBackend: true,
		},
		{
			name:                 "get with custom backend path",
			appBackendPathPrefix: "/svc",
			endpoint: &endpointModel.Endpoint{
				Method: http.MethodGet,
				Path:   "users",
				Backend: endpointModel.Backend{
					CustomPath: "profiles",
				},
			},
			requestURL:   "https://public.example/api/users?q=1",
			wantStatus:   http.StatusNoContent,
			wantMethod:   http.MethodGet,
			wantPath:     "/svc/profiles",
			wantRawQuery: "q=1",
			checkBackend: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := New(&rootModel.Root{
				BaseUrl: "https://public.example",
				Apps: []*appModel.App{
					{
						PathPrefix: "api",
						Backend: appModel.AppBackend{
							Url: backend.URL + tt.appBackendPathPrefix,
						},
						Endpoints: []*endpointModel.Endpoint{
							tt.endpoint,
						},
					},
				},
			}, nil)
			require.NoError(t, err)

			rw := httptest.NewRecorder()

			s.ServeHTTP(rw, httptest.NewRequest(tt.endpoint.Method, tt.requestURL, nil))

			require.Equal(t, tt.wantStatus, rw.Code)
			require.Equal(t, tt.wantMethod, rw.Header().Get("X-Backend-Method"))
			require.Equal(t, tt.wantPath, rw.Header().Get("X-Backend-Path"))
			require.Equal(t, tt.wantRawQuery, rw.Header().Get("X-Backend-Query"))
		})
	}
}

func TestServiceBuild_DuplicateRoute(t *testing.T) {
	_, err := New(&rootModel.Root{
		BaseUrl: "https://public.example",
		Apps: []*appModel.App{
			{
				PathPrefix: "api",
				Backend: appModel.AppBackend{
					Url: "http://example.local/svc",
				},
				Endpoints: []*endpointModel.Endpoint{
					{
						Method: http.MethodGet,
						Path:   "users",
					},
					{
						Method: http.MethodGet,
						Path:   "users",
					},
				},
			},
		},
	}, nil)
	require.Error(t, err)
}

func TestServiceBuild_Auth(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer backend.Close()

	tests := []struct {
		name       string
		endpoint   *endpointModel.Endpoint
		request    func() *http.Request
		wantStatus int
	}{
		{
			name: "auth disabled",
			endpoint: &endpointModel.Endpoint{
				Method: http.MethodGet,
				Path:   "users",
				Auth: endpointModel.Auth{
					Enabled: false,
				},
			},
			request: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "https://public.example/api/users", nil)
			},
			wantStatus: http.StatusNoContent,
		},
		{
			name: "basic auth success",
			endpoint: &endpointModel.Endpoint{
				Method: http.MethodGet,
				Path:   "users",
				Auth: endpointModel.Auth{
					Enabled: true,
					Methods: []endpointModel.AuthMethod{
						{
							Basic: &endpointModel.AuthMethodBasic{
								Users: []endpointModel.AuthMethodBasicUser{
									{Username: "admin", Password: "qwerty"},
								},
							},
						},
					},
				},
			},
			request: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "https://public.example/api/users", nil)
				req.SetBasicAuth("admin", "qwerty")
				return req
			},
			wantStatus: http.StatusNoContent,
		},
		{
			name: "auth required and all methods failed",
			endpoint: &endpointModel.Endpoint{
				Method: http.MethodGet,
				Path:   "users",
				Auth: endpointModel.Auth{
					Enabled: true,
					Methods: []endpointModel.AuthMethod{
						{
							Basic: &endpointModel.AuthMethodBasic{
								Users: []endpointModel.AuthMethodBasicUser{
									{Username: "admin", Password: "qwerty"},
								},
							},
						},
						{
							APIKey: &endpointModel.AuthMethodAPIKey{
								Header: "X-API-Key",
								Keys:   []string{"k-1"},
							},
						},
					},
				},
			},
			request: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "https://public.example/api/users", nil)
			},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name: "auth required and one of methods succeeded",
			endpoint: &endpointModel.Endpoint{
				Method: http.MethodGet,
				Path:   "users",
				Auth: endpointModel.Auth{
					Enabled: true,
					Methods: []endpointModel.AuthMethod{
						{
							Basic: &endpointModel.AuthMethodBasic{
								Users: []endpointModel.AuthMethodBasicUser{
									{Username: "admin", Password: "qwerty"},
								},
							},
						},
						{
							APIKey: &endpointModel.AuthMethodAPIKey{
								Header: "X-API-Key",
								Keys:   []string{"k-1"},
							},
						},
					},
				},
			},
			request: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "https://public.example/api/users", nil)
				req.Header.Set("X-API-Key", "k-1")
				return req
			},
			wantStatus: http.StatusNoContent,
		},
		{
			name: "api key default header",
			endpoint: &endpointModel.Endpoint{
				Method: http.MethodGet,
				Path:   "users",
				Auth: endpointModel.Auth{
					Enabled: true,
					Methods: []endpointModel.AuthMethod{
						{
							APIKey: &endpointModel.AuthMethodAPIKey{
								Keys: []string{"k-1"},
							},
						},
					},
				},
			},
			request: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "https://public.example/api/users", nil)
				req.Header.Set("X-API-Key", "k-1")
				return req
			},
			wantStatus: http.StatusNoContent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := New(&rootModel.Root{
				BaseUrl: "https://public.example",
				Apps: []*appModel.App{
					{
						PathPrefix: "api",
						Backend: appModel.AppBackend{
							Url: backend.URL + "/svc",
						},
						Endpoints: []*endpointModel.Endpoint{
							tt.endpoint,
						},
					},
				},
			}, nil)
			require.NoError(t, err)

			rw := httptest.NewRecorder()
			s.ServeHTTP(rw, tt.request())

			require.Equal(t, tt.wantStatus, rw.Code)
		})
	}
}
