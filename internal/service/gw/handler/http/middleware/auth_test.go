package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	authModel "github.com/rendau/ruto/internal/domain/auth/model"
	domEndpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
)

func TestAuthStatusCodes(t *testing.T) {
	ep := &domEndpointModel.Endpoint{
		Auth: authModel.Auth{
			Enabled: true,
			Methods: []*authModel.AuthMethod{
				{
					APIKey: &authModel.AuthMethodAPIKey{
						Keys: []authModel.AuthMethodAPIKeyItem{{Name: "partner", Key: "secret-key"}},
					},
					IPValidation: &authModel.AuthMethodIPValidation{
						AllowedIps: []authModel.AuthMethodIPValidationItem{{Ip: "10.0.0.1"}},
					},
				},
			},
		},
	}

	handler := NewAuth(ep)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	tests := []struct {
		name       string
		apiKey     string
		remoteAddr string
		want       int
	}{
		{"allowed", "secret-key", "10.0.0.1:1234", http.StatusOK},
		{"no api key", "", "10.0.0.1:1234", http.StatusUnauthorized},
		{"wrong api key", "wrong", "10.0.0.1:1234", http.StatusUnauthorized},
		{"valid api key from not allowed ip", "secret-key", "10.0.0.2:1234", http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tt.remoteAddr
			if tt.apiKey != "" {
				req.Header.Set("X-Api-Key", tt.apiKey)
			}

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			require.Equal(t, tt.want, rec.Code)
		})
	}
}
