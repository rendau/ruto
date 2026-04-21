package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/rendau/ruto/internal/model/config"
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
		endpoint             *config.Endpoint
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
			endpoint: &config.Endpoint{
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
			endpoint: &config.Endpoint{
				Method: http.MethodGet,
				Path:   "users",
				Backend: config.EndpointBackend{
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
			s := New()
			err := s.Build(&config.Root{
				PublicBaseUrl: "https://public.example",
				Apps: []*config.App{
					{
						PublicPathPrefix: "api",
						Backend: config.AppBackend{
							UrlStr: backend.URL + tt.appBackendPathPrefix,
						},
						Endpoints: []*config.Endpoint{
							tt.endpoint,
						},
					},
				},
			})
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
	s := New()

	err := s.Build(&config.Root{
		PublicBaseUrl: "https://public.example",
		Apps: []*config.App{
			{
				PublicPathPrefix: "api",
				Backend: config.AppBackend{
					UrlStr: "http://example.local/svc",
				},
				Endpoints: []*config.Endpoint{
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
	})
	require.Error(t, err)
}
