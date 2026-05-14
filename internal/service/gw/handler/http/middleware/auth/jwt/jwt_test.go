package jwt

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"math/big"
	"net/http/httptest"
	"testing"

	jwtv5 "github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"

	endpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
	"github.com/rendau/ruto/internal/service/gw/handler/http/request"
	"github.com/rendau/ruto/internal/service/gw/jwk"
)

type testJWKServiceT struct {
	items map[string]*jwk.Item
}

func (s *testJWKServiceT) Get(kid string) *jwk.Item {
	return s.items[kid]
}

func TestAuthorize(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	publicKey := privateKey.PublicKey

	item := &jwk.Item{
		Kid: "kid-1",
		Alg: jwtv5.SigningMethodRS256.Alg(),
		N:   base64.RawURLEncoding.EncodeToString(publicKey.N.Bytes()),
		E:   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(publicKey.E)).Bytes()),
	}

	signToken := func(method jwtv5.SigningMethod, kid string, claims jwtv5.MapClaims, key any) string {
		tokenObj := jwtv5.NewWithClaims(method, claims)
		if kid != "" {
			tokenObj.Header["kid"] = kid
		}

		token, signErr := tokenObj.SignedString(key)
		require.NoError(t, signErr)
		return token
	}

	validToken := signToken(
		jwtv5.SigningMethodRS256,
		item.Kid,
		jwtv5.MapClaims{"sub": "u1", "roles": []string{"moderator", "admin"}},
		privateKey,
	)
	keycloakResourceAccessToken := signToken(
		jwtv5.SigningMethodRS256,
		item.Kid,
		jwtv5.MapClaims{
			"sub": "u1",
			"resource_access": map[string]any{
				"ruto": map[string]any{
					"roles": []any{"admin", "moderator"},
				},
			},
		},
		privateKey,
	)

	wrongKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	badSignatureToken := signToken(
		jwtv5.SigningMethodRS256,
		item.Kid,
		jwtv5.MapClaims{"sub": "u1"},
		wrongKey,
	)

	missingKidToken := signToken(
		jwtv5.SigningMethodRS256,
		"",
		jwtv5.MapClaims{"sub": "u1"},
		privateKey,
	)

	hsToken := signToken(
		jwtv5.SigningMethodHS256,
		item.Kid,
		jwtv5.MapClaims{"sub": "u1"},
		[]byte("secret"),
	)

	testJWKService := &testJWKServiceT{items: map[string]*jwk.Item{item.Kid: item}}

	tests := []struct {
		name   string
		conf   *endpointModel.AuthMethodJWT
		header string
		ctxReq *request.Request
		want   bool
	}{
		{
			name:   "missing token",
			conf:   &endpointModel.AuthMethodJWT{},
			header: "",
			ctxReq: &request.Request{JwkService: testJWKService},
			want:   false,
		},
		{
			name:   "invalid algorithm",
			conf:   &endpointModel.AuthMethodJWT{},
			header: "Bearer " + hsToken,
			ctxReq: &request.Request{JwkService: testJWKService},
			want:   false,
		},
		{
			name:   "missing kid",
			conf:   &endpointModel.AuthMethodJWT{},
			header: "Bearer " + missingKidToken,
			ctxReq: &request.Request{JwkService: testJWKService},
			want:   false,
		},
		{
			name:   "kid is not allowed by config",
			conf:   &endpointModel.AuthMethodJWT{Kids: []string{"kid-2"}},
			header: "Bearer " + validToken,
			ctxReq: &request.Request{JwkService: testJWKService},
			want:   false,
		},
		{
			name:   "kid not found in jwk service",
			conf:   &endpointModel.AuthMethodJWT{},
			header: "Bearer " + validToken,
			ctxReq: &request.Request{JwkService: &testJWKServiceT{items: map[string]*jwk.Item{}}},
			want:   false,
		},
		{
			name:   "jwk alg mismatch",
			conf:   &endpointModel.AuthMethodJWT{},
			header: "Bearer " + validToken,
			ctxReq: &request.Request{JwkService: &testJWKServiceT{items: map[string]*jwk.Item{
				item.Kid: {
					Kid: item.Kid,
					Alg: jwtv5.SigningMethodRS384.Alg(),
					N:   item.N,
					E:   item.E,
				},
			}}},
			want: false,
		},
		{
			name:   "invalid signature",
			conf:   &endpointModel.AuthMethodJWT{},
			header: "Bearer " + badSignatureToken,
			ctxReq: &request.Request{JwkService: testJWKService},
			want:   false,
		},
		{
			name:   "required role missing",
			conf:   &endpointModel.AuthMethodJWT{Roles: []string{"super-admin"}},
			header: "Bearer " + validToken,
			ctxReq: &request.Request{JwkService: testJWKService},
			want:   false,
		},
		{
			name:   "authorized",
			conf:   &endpointModel.AuthMethodJWT{Roles: []string{"moderator"}},
			header: "Bearer " + validToken,
			ctxReq: &request.Request{JwkService: testJWKService},
			want:   true,
		},
		{
			name:   "authorized by keycloak resource_access roles",
			conf:   &endpointModel.AuthMethodJWT{Roles: []string{"admin"}},
			header: "Bearer " + keycloakResourceAccessToken,
			ctxReq: &request.Request{JwkService: testJWKService},
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "http://localhost", nil)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}
			if tt.ctxReq != nil {
				req = req.WithContext(request.Inject(context.Background(), tt.ctxReq))
			}

			got := New(tt.conf).Authorize(req)
			require.Equal(t, tt.want, got)
		})
	}
}
