package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	endpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
)

func TestNew_ORAndANDCases(t *testing.T) {
	tests := []struct {
		name       string
		endpoint   *endpointModel.Endpoint
		request    func() *http.Request
		wantStatus int
	}{
		{
			name: "or second method passes",
			endpoint: &endpointModel.Endpoint{
				Auth: endpointModel.Auth{
					Enabled: true,
					Methods: []endpointModel.AuthMethod{
						{
							APIKey: &endpointModel.AuthMethodAPIKey{
								Header: "X-API-Key",
								Keys:   []string{"k-1"},
							},
						},
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
				req := httptest.NewRequest(http.MethodGet, "https://public.example/test", nil)
				req.SetBasicAuth("admin", "qwerty")
				return req
			},
			wantStatus: http.StatusNoContent,
		},
		{
			name: "and passes when all checks pass",
			endpoint: &endpointModel.Endpoint{
				Auth: endpointModel.Auth{
					Enabled: true,
					Methods: []endpointModel.AuthMethod{
						{
							APIKey: &endpointModel.AuthMethodAPIKey{
								Header: "X-API-Key",
								Keys:   []string{"k-1"},
							},
							IPValidation: &endpointModel.AuthMethodIPValidation{
								AllowedIps: []string{"192.0.2.1"},
							},
						},
					},
				},
			},
			request: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "https://public.example/test", nil)
				req.Header.Set("X-API-Key", "k-1")
				req.RemoteAddr = "192.0.2.1:1234"
				return req
			},
			wantStatus: http.StatusNoContent,
		},
		{
			name: "and fails when api key fails",
			endpoint: &endpointModel.Endpoint{
				Auth: endpointModel.Auth{
					Enabled: true,
					Methods: []endpointModel.AuthMethod{
						{
							APIKey: &endpointModel.AuthMethodAPIKey{
								Header: "X-API-Key",
								Keys:   []string{"k-1"},
							},
							IPValidation: &endpointModel.AuthMethodIPValidation{
								AllowedIps: []string{"192.0.2.1"},
							},
						},
					},
				},
			},
			request: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "https://public.example/test", nil)
				req.Header.Set("X-API-Key", "wrong")
				req.RemoteAddr = "192.0.2.1:1234"
				return req
			},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name: "and fails when ip check fails",
			endpoint: &endpointModel.Endpoint{
				Auth: endpointModel.Auth{
					Enabled: true,
					Methods: []endpointModel.AuthMethod{
						{
							APIKey: &endpointModel.AuthMethodAPIKey{
								Header: "X-API-Key",
								Keys:   []string{"k-1"},
							},
							IPValidation: &endpointModel.AuthMethodIPValidation{
								AllowedIps: []string{"10.0.0.1"},
							},
						},
					},
				},
			},
			request: func() *http.Request {
				req := httptest.NewRequest(http.MethodGet, "https://public.example/test", nil)
				req.Header.Set("X-API-Key", "k-1")
				req.RemoteAddr = "192.0.2.1:1234"
				return req
			},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name: "or with composite first method failed but second passed",
			endpoint: &endpointModel.Endpoint{
				Auth: endpointModel.Auth{
					Enabled: true,
					Methods: []endpointModel.AuthMethod{
						{
							APIKey: &endpointModel.AuthMethodAPIKey{
								Header: "X-API-Key",
								Keys:   []string{"k-1"},
							},
							IPValidation: &endpointModel.AuthMethodIPValidation{
								AllowedIps: []string{"10.0.0.1"},
							},
						},
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
				req := httptest.NewRequest(http.MethodGet, "https://public.example/test", nil)
				req.SetBasicAuth("admin", "qwerty")
				req.RemoteAddr = "192.0.2.1:1234"
				return req
			},
			wantStatus: http.StatusNoContent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rw := httptest.NewRecorder()
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			})

			h := New(tt.endpoint)(next)
			h.ServeHTTP(rw, tt.request())

			require.Equal(t, tt.wantStatus, rw.Code)
		})
	}
}
