package middleware

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	jwtv5 "github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"

	authModel "github.com/rendau/ruto/internal/domain/auth/model"
	domEndpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
	rootModel "github.com/rendau/ruto/internal/domain/root/model"
	"github.com/rendau/ruto/internal/service/gw/service/jwk"
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

func TestAuthCookieTokenGoesToBackend(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	const kid = "cookie-kid"

	jwks := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(jwk.ItemSet{Keys: []*jwk.Item{{
			Kty: "RSA",
			Use: "sig",
			Alg: "RS256",
			Kid: kid,
			N:   base64.RawURLEncoding.EncodeToString(privateKey.N.Bytes()),
			E:   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(privateKey.E)).Bytes()),
		}}})
	}))
	defer jwks.Close()

	jwk.Ins().SetProviders([]rootModel.RootJwt{{Name: "corp", JwkUrl: jwks.URL}})
	defer jwk.Ins().SetProviders(nil)
	require.Eventually(t, func() bool {
		key, _ := jwk.Ins().GetPublicKey("corp", kid)
		return key != nil
	}, 2*time.Second, 20*time.Millisecond)

	tokenObj := jwtv5.NewWithClaims(jwtv5.SigningMethodRS256, jwtv5.MapClaims{"sub": "u1"})
	tokenObj.Header["kid"] = kid
	token, err := tokenObj.SignedString(privateKey)
	require.NoError(t, err)

	ep := &domEndpointModel.Endpoint{
		Auth: authModel.Auth{
			Enabled: true,
			Methods: []*authModel.AuthMethod{{JWT: &authModel.AuthMethodJWT{Provider: "corp"}}},
		},
	}

	var seen http.Header
	handler := NewAuth(ep)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
	}))

	tests := []struct {
		name              string
		authorization     string
		cookie            string
		wantCode          int
		wantAuthorization string
	}{
		{
			name:              "cookie token: backend gets both the cookie and a bearer",
			cookie:            "ruto_corp=" + token + "; theme=dark",
			wantCode:          http.StatusOK,
			wantAuthorization: "Bearer " + token,
		},
		{
			name:              "bearer in the header stays as sent",
			authorization:     "Bearer " + token,
			cookie:            "ruto_corp=" + token,
			wantCode:          http.StatusOK,
			wantAuthorization: "Bearer " + token,
		},
		{
			name:     "cookie of an unknown provider is ignored",
			cookie:   "ruto_other=" + token,
			wantCode: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seen = nil
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.authorization != "" {
				req.Header.Set("Authorization", tt.authorization)
			}
			req.Header.Set("Cookie", tt.cookie)

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			require.Equal(t, tt.wantCode, rec.Code)
			if tt.wantCode != http.StatusOK {
				require.Nil(t, seen)
				return
			}
			require.Equal(t, tt.wantAuthorization, seen.Get("Authorization"))
			require.Equal(t, tt.cookie, seen.Get("Cookie"), "the cookie is forwarded untouched")
		})
	}
}
