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
	varsModel "github.com/rendau/ruto/internal/domain/vars/model"
)

func TestService_HTTPRouteMatchingAndProxying(t *testing.T) {
	tests := []struct {
		name              string
		endpoint          *endpointModel.Endpoint
		requestMethod     string
		requestPath       string
		wantBackendMethod string
		wantBackendPath   string
		wantBackendQuery  string
	}{
		{
			name: "GET endpoint forwards with app prefix stripped",
			endpoint: &endpointModel.Endpoint{
				Active: true,
				Type:   endpointModel.TypeHTTP,
				Http: endpointModel.Http{
					Method: http.MethodGet,
					Path:   "profile",
				},
			},
			requestMethod:     http.MethodGet,
			requestPath:       "/account/profile?page=1",
			wantBackendMethod: http.MethodGet,
			wantBackendPath:   "/profile",
			wantBackendQuery:  "page=1",
		},
		{
			name: "empty endpoint type is treated as HTTP",
			endpoint: &endpointModel.Endpoint{
				Active: true,
				Http: endpointModel.Http{
					Method: http.MethodGet,
					Path:   "profile",
				},
			},
			requestMethod:     http.MethodGet,
			requestPath:       "/account/profile",
			wantBackendMethod: http.MethodGet,
			wantBackendPath:   "/profile",
		},
		{
			name: "wildcard method accepts non GET request",
			endpoint: &endpointModel.Endpoint{
				Active: true,
				Type:   endpointModel.TypeHTTP,
				Http: endpointModel.Http{
					Method: "*",
					Path:   "profile",
				},
			},
			requestMethod:     http.MethodPatch,
			requestPath:       "/account/profile",
			wantBackendMethod: http.MethodPatch,
			wantBackendPath:   "/profile",
		},
		{
			name: "custom backend path overrides forwarded path",
			endpoint: &endpointModel.Endpoint{
				Active: true,
				Type:   endpointModel.TypeHTTP,
				Http: endpointModel.Http{
					Method: http.MethodGet,
					Path:   "profile",
				},
				Backend: endpointModel.Backend{
					CustomPath: "internal/me",
				},
			},
			requestMethod:     http.MethodGet,
			requestPath:       "/account/profile?expand=roles",
			wantBackendMethod: http.MethodGet,
			wantBackendPath:   "/internal/me",
			wantBackendQuery:  "expand=roles",
		},
		{
			name: "chi path params are forwarded as request path",
			endpoint: &endpointModel.Endpoint{
				Active: true,
				Type:   endpointModel.TypeHTTP,
				Http: endpointModel.Http{
					Method: http.MethodGet,
					Path:   "users/{id}",
				},
			},
			requestMethod:     http.MethodGet,
			requestPath:       "/account/users/42",
			wantBackendMethod: http.MethodGet,
			wantBackendPath:   "/users/42",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backendHit := false
			backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				backendHit = true
				require.Equal(t, tt.wantBackendMethod, r.Method)
				require.Equal(t, tt.wantBackendPath, r.URL.Path)
				require.Equal(t, tt.wantBackendQuery, r.URL.RawQuery)
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("ok"))
			}))
			defer backend.Close()

			service := newTestService(t, backend.URL, true, tt.endpoint, nil)

			rec := performRequest(service, tt.requestMethod, tt.requestPath, nil)

			require.True(t, backendHit)
			require.Equal(t, http.StatusOK, rec.Code)
			require.Equal(t, "ok", rec.Body.String())
		})
	}
}

func TestService_HTTPBackendRequestParams(t *testing.T) {
	backendHit := false
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		backendHit = true
		require.Equal(t, "/profile", r.URL.Path)
		require.Equal(t, "app_only=1&ep_only=2&inbound=ok&shared=app-tenant", r.URL.RawQuery)
		require.Equal(t, "endpoint-token:app", r.Header.Get("X-App-Token"))
		require.Equal(t, "endpoint-token", r.Header.Get("X-Endpoint-Token"))
		require.Equal(t, "endpoint", r.Header.Get("X-Shared"))
		require.Equal(t, "client", r.Header.Get("X-Client"))
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	snapshot := rootModel.NewEmpty()
	snapshot.Variables = varsModel.Vars{
		"token":  "root-token",
		"tenant": "root-tenant",
	}
	snapshot.Apps = []*appModel.App{
		{
			Active:     true,
			PathPrefix: "/account",
			Name:       "account",
			Variables: varsModel.Vars{
				"tenant":    "app-tenant",
				"app_token": "{{token}}:app",
			},
			Backend: appModel.Backend{
				Url: backend.URL,
				Headers: varsModel.Vars{
					"X-App-Token": "{{app_token}}",
					"X-Shared":    "app",
				},
				QueryParams: varsModel.Vars{
					"app_only": "1",
					"shared":   "app",
				},
			},
			Endpoints: []*endpointModel.Endpoint{
				{
					Active: true,
					Type:   endpointModel.TypeHTTP,
					Http: endpointModel.Http{
						Method: http.MethodGet,
						Path:   "profile",
					},
					Variables: varsModel.Vars{
						"token": "endpoint-token",
					},
					Backend: endpointModel.Backend{
						Headers: varsModel.Vars{
							"X-Endpoint-Token": "{{token}}",
							"X-Shared":         "endpoint",
						},
						QueryParams: varsModel.Vars{
							"ep_only": "2",
							"shared":  "{{tenant}}",
						},
					},
				},
			},
		},
	}
	require.NoError(t, snapshot.Normalize())
	snapshot.InheritDown()
	snapshot.Interpolate()

	service, err := New(snapshot)
	require.NoError(t, err)

	rec := performRequest(service, http.MethodGet, "/account/profile?inbound=ok&shared=inbound", http.Header{
		"X-Client": []string{"client"},
	})

	require.True(t, backendHit)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestService_RouteExclusionAndMethodHandling(t *testing.T) {
	tests := []struct {
		name           string
		appActive      bool
		endpoint       *endpointModel.Endpoint
		requestMethod  string
		requestPath    string
		wantStatusCode int
	}{
		{
			name:      "inactive app is not registered",
			appActive: false,
			endpoint: &endpointModel.Endpoint{
				Active: true,
				Type:   endpointModel.TypeHTTP,
				Http: endpointModel.Http{
					Method: http.MethodGet,
					Path:   "profile",
				},
			},
			requestMethod:  http.MethodGet,
			requestPath:    "/account/profile",
			wantStatusCode: http.StatusNotFound,
		},
		{
			name:      "inactive endpoint is not registered",
			appActive: true,
			endpoint: &endpointModel.Endpoint{
				Active: false,
				Type:   endpointModel.TypeHTTP,
				Http: endpointModel.Http{
					Method: http.MethodGet,
					Path:   "profile",
				},
			},
			requestMethod:  http.MethodGet,
			requestPath:    "/account/profile",
			wantStatusCode: http.StatusNotFound,
		},
		{
			name:      "grpc endpoint is not registered in HTTP gateway",
			appActive: true,
			endpoint: &endpointModel.Endpoint{
				Active: true,
				Type:   endpointModel.TypeGRPC,
				Grpc: endpointModel.Grpc{
					Service: "account.AccountService",
					Method:  "Profile",
					Path:    "/account.AccountService/Profile",
				},
			},
			requestMethod:  http.MethodGet,
			requestPath:    "/account/account.AccountService/Profile",
			wantStatusCode: http.StatusNotFound,
		},
		{
			name:      "method mismatch returns method not allowed",
			appActive: true,
			endpoint: &endpointModel.Endpoint{
				Active: true,
				Type:   endpointModel.TypeHTTP,
				Http: endpointModel.Http{
					Method: http.MethodGet,
					Path:   "profile",
				},
			},
			requestMethod:  http.MethodPost,
			requestPath:    "/account/profile",
			wantStatusCode: http.StatusMethodNotAllowed,
		},
		{
			name:      "unknown path returns not found",
			appActive: true,
			endpoint: &endpointModel.Endpoint{
				Active: true,
				Type:   endpointModel.TypeHTTP,
				Http: endpointModel.Http{
					Method: http.MethodGet,
					Path:   "profile",
				},
			},
			requestMethod:  http.MethodGet,
			requestPath:    "/account/missing",
			wantStatusCode: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backendHit := false
			backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				backendHit = true
				w.WriteHeader(http.StatusOK)
			}))
			defer backend.Close()

			service := newTestService(t, backend.URL, tt.appActive, tt.endpoint, nil)

			rec := performRequest(service, tt.requestMethod, tt.requestPath, nil)

			require.False(t, backendHit)
			require.Equal(t, tt.wantStatusCode, rec.Code)
		})
	}
}

func TestService_AuthMiddleware(t *testing.T) {
	tests := []struct {
		name           string
		headers        http.Header
		wantStatusCode int
		wantBackendHit bool
	}{
		{
			name:           "missing api key is unauthorized",
			wantStatusCode: http.StatusUnauthorized,
			wantBackendHit: false,
		},
		{
			name: "valid api key is forwarded",
			headers: http.Header{
				"X-API-Key": []string{"secret"},
			},
			wantStatusCode: http.StatusOK,
			wantBackendHit: true,
		},
		{
			name: "invalid api key is unauthorized",
			headers: http.Header{
				"X-API-Key": []string{"wrong"},
			},
			wantStatusCode: http.StatusUnauthorized,
			wantBackendHit: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backendHit := false
			backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				backendHit = true
				w.WriteHeader(http.StatusOK)
			}))
			defer backend.Close()

			endpoint := &endpointModel.Endpoint{
				Active: true,
				Type:   endpointModel.TypeHTTP,
				Http: endpointModel.Http{
					Method: http.MethodGet,
					Path:   "profile",
				},
				Auth: authModel.Auth{
					Enabled: true,
					Mode:    constant.AuthModeReplace,
					Methods: []*authModel.AuthMethod{
						{
							APIKey: &authModel.AuthMethodAPIKey{
								Header: "X-API-Key",
								Keys:   []authModel.AuthMethodAPIKeyItem{{Key: "secret"}},
							},
						},
					},
				},
			}
			service := newTestService(t, backend.URL, true, endpoint, nil)

			rec := performRequest(service, http.MethodGet, "/account/profile", tt.headers)

			require.Equal(t, tt.wantBackendHit, backendHit)
			require.Equal(t, tt.wantStatusCode, rec.Code)
		})
	}
}

func TestService_CorsMiddleware(t *testing.T) {
	tests := []struct {
		name             string
		cors             *rootModel.RootCors
		requestMethod    string
		requestPath      string
		headers          http.Header
		wantStatusCode   int
		wantBackendHit   bool
		wantAllowOrigin  string
		wantAllowMethods string
		wantAllowHeaders string
		wantAllowCreds   string
		wantMaxAge       string
		wantVaryContains []string
	}{
		{
			name:           "disabled cors forwards without cors headers",
			requestMethod:  http.MethodGet,
			requestPath:    "/account/profile",
			headers:        http.Header{"Origin": []string{"https://front.example"}},
			wantStatusCode: http.StatusOK,
			wantBackendHit: true,
		},
		{
			name: "allowed actual request gets cors headers",
			cors: &rootModel.RootCors{
				Enabled:          true,
				AllowCredentials: true,
				MaxAge:           "600",
				AllowOrigins:     []string{"https://front.example"},
				AllowMethods:     []string{http.MethodGet, http.MethodPost},
				AllowHeaders:     []string{"Authorization", "X-API-Key"},
			},
			requestMethod:    http.MethodGet,
			requestPath:      "/account/profile",
			headers:          http.Header{"Origin": []string{"https://front.example"}},
			wantStatusCode:   http.StatusOK,
			wantBackendHit:   true,
			wantAllowOrigin:  "https://front.example",
			wantAllowCreds:   "true",
			wantVaryContains: []string{"Origin"},
		},
		{
			name: "allowed preflight is handled before router",
			cors: &rootModel.RootCors{
				Enabled:          true,
				AllowCredentials: false,
				MaxAge:           "600",
				AllowOrigins:     []string{"*"},
				AllowMethods:     []string{http.MethodGet, http.MethodPost},
				AllowHeaders:     []string{"Authorization", "X-API-Key"},
			},
			requestMethod: http.MethodOptions,
			requestPath:   "/account/profile",
			headers: http.Header{
				"Origin":                        []string{"https://front.example"},
				"Access-Control-Request-Method": []string{http.MethodGet},
			},
			wantStatusCode:   http.StatusNoContent,
			wantBackendHit:   false,
			wantAllowOrigin:  "*",
			wantAllowMethods: "GET, POST",
			wantAllowHeaders: "Authorization, X-API-Key",
			wantMaxAge:       "600",
			wantVaryContains: []string{"Origin", "Access-Control-Request-Method", "Access-Control-Request-Headers"},
		},
		{
			name: "disallowed preflight origin is rejected before router",
			cors: &rootModel.RootCors{
				Enabled:      true,
				AllowOrigins: []string{"https://front.example"},
				AllowMethods: []string{http.MethodGet},
			},
			requestMethod: http.MethodOptions,
			requestPath:   "/account/profile",
			headers: http.Header{
				"Origin":                        []string{"https://evil.example"},
				"Access-Control-Request-Method": []string{http.MethodGet},
			},
			wantStatusCode: http.StatusForbidden,
			wantBackendHit: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backendHit := false
			backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				backendHit = true
				w.WriteHeader(http.StatusOK)
			}))
			defer backend.Close()

			endpoint := &endpointModel.Endpoint{
				Active: true,
				Type:   endpointModel.TypeHTTP,
				Http: endpointModel.Http{
					Method: http.MethodGet,
					Path:   "profile",
				},
			}
			service := newTestService(t, backend.URL, true, endpoint, tt.cors)

			rec := performRequest(service, tt.requestMethod, tt.requestPath, tt.headers)

			require.Equal(t, tt.wantBackendHit, backendHit)
			require.Equal(t, tt.wantStatusCode, rec.Code)
			require.Equal(t, tt.wantAllowOrigin, rec.Header().Get("Access-Control-Allow-Origin"))
			require.Equal(t, tt.wantAllowMethods, rec.Header().Get("Access-Control-Allow-Methods"))
			require.Equal(t, tt.wantAllowHeaders, rec.Header().Get("Access-Control-Allow-Headers"))
			require.Equal(t, tt.wantAllowCreds, rec.Header().Get("Access-Control-Allow-Credentials"))
			require.Equal(t, tt.wantMaxAge, rec.Header().Get("Access-Control-Max-Age"))
			for _, wantVary := range tt.wantVaryContains {
				require.Contains(t, rec.Header().Values("Vary"), wantVary)
			}
		})
	}
}

func TestService_RedirectLocationRewrite(t *testing.T) {
	tests := []struct {
		name         string
		location     string
		wantLocation string
	}{
		{
			name:         "absolute-path redirect is prefixed with app path",
			location:     "/login",
			wantLocation: "/account/login",
		},
		{
			name:         "relative redirect is kept untouched",
			location:     "login",
			wantLocation: "login",
		},
		{
			name:         "absolute redirect is kept untouched",
			location:     "https://auth.example/login",
			wantLocation: "https://auth.example/login",
		},
		{
			name:         "protocol-relative redirect is kept untouched",
			location:     "//auth.example/login",
			wantLocation: "//auth.example/login",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Location", tt.location)
				w.WriteHeader(http.StatusFound)
			}))
			defer backend.Close()

			endpoint := &endpointModel.Endpoint{
				Active: true,
				Type:   endpointModel.TypeHTTP,
				Http: endpointModel.Http{
					Method: http.MethodGet,
					Path:   "profile",
				},
			}
			service := newTestService(t, backend.URL, true, endpoint, nil)

			rec := performRequest(service, http.MethodGet, "/account/profile", nil)

			require.Equal(t, http.StatusFound, rec.Code)
			require.Equal(t, tt.wantLocation, rec.Header().Get("Location"))
		})
	}
}

func newTestService(
	t *testing.T,
	backendURL string,
	appActive bool,
	endpoint *endpointModel.Endpoint,
	cors *rootModel.RootCors,
) *Service {
	t.Helper()

	snapshot := rootModel.NewEmpty()
	if cors != nil {
		snapshot.Cors = *cors
	}
	snapshot.Apps = []*appModel.App{
		{
			Active:     appActive,
			PathPrefix: "/account",
			Name:       "account",
			Backend: appModel.Backend{
				Url: backendURL,
			},
			Endpoints: []*endpointModel.Endpoint{endpoint},
		},
	}
	require.NoError(t, snapshot.Normalize())
	snapshot.InheritDown()
	snapshot.Interpolate()

	service, err := New(snapshot)
	require.NoError(t, err)
	return service
}

func performRequest(service *Service, method string, path string, headers http.Header) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	for key, values := range headers {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}
	rec := httptest.NewRecorder()
	service.ServeHTTP(rec, req)
	return rec
}
