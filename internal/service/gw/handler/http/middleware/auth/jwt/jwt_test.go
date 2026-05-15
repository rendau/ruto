package jwt

import (
	"crypto/rand"
	"crypto/rsa"
	"net/http/httptest"
	"testing"

	jwtv5 "github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"

	authModel "github.com/rendau/ruto/internal/domain/auth/model"
)

type testJWKGetterT struct {
	keys map[string]struct {
		publicKey *rsa.PublicKey
		alg       string
	}
}

func (s *testJWKGetterT) GetPublicKey(kid string) (*rsa.PublicKey, string) {
	item := s.keys[kid]
	return item.publicKey, item.alg
}

func TestAuthorize(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	publicKey := privateKey.PublicKey

	itemKid := "kid-1"
	itemAlg := jwtv5.SigningMethodRS256.Alg()

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
		itemKid,
		jwtv5.MapClaims{"sub": "u1", "roles": []string{"moderator", "admin"}},
		privateKey,
	)
	keycloakResourceAccessToken := signToken(
		jwtv5.SigningMethodRS256,
		itemKid,
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
		itemKid,
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
		itemKid,
		jwtv5.MapClaims{"sub": "u1"},
		[]byte("secret"),
	)

	testJWKGetter := &testJWKGetterT{
		keys: map[string]struct {
			publicKey *rsa.PublicKey
			alg       string
		}{
			itemKid: {
				publicKey: &publicKey,
				alg:       itemAlg,
			},
		},
	}

	tests := []struct {
		name      string
		conf      *authModel.AuthMethodJWT
		header    string
		jwkGetter JwkGetterI
		want      bool
	}{
		{
			name:      "missing token",
			conf:      &authModel.AuthMethodJWT{},
			header:    "",
			jwkGetter: testJWKGetter,
			want:      false,
		},
		{
			name:      "invalid algorithm",
			conf:      &authModel.AuthMethodJWT{},
			header:    "Bearer " + hsToken,
			jwkGetter: testJWKGetter,
			want:      false,
		},
		{
			name:      "missing kid",
			conf:      &authModel.AuthMethodJWT{},
			header:    "Bearer " + missingKidToken,
			jwkGetter: testJWKGetter,
			want:      false,
		},
		{
			name:      "kid is not allowed by config",
			conf:      &authModel.AuthMethodJWT{Kids: []string{"kid-2"}},
			header:    "Bearer " + validToken,
			jwkGetter: testJWKGetter,
			want:      false,
		},
		{
			name:      "kid not found in jwk service",
			conf:      &authModel.AuthMethodJWT{},
			header:    "Bearer " + validToken,
			jwkGetter: &testJWKGetterT{},
			want:      false,
		},
		{
			name:   "jwk alg mismatch",
			conf:   &authModel.AuthMethodJWT{},
			header: "Bearer " + validToken,
			jwkGetter: &testJWKGetterT{
				keys: map[string]struct {
					publicKey *rsa.PublicKey
					alg       string
				}{
					itemKid: {
						publicKey: &publicKey,
						alg:       jwtv5.SigningMethodRS384.Alg(),
					},
				},
			},
			want: false,
		},
		{
			name:      "invalid signature",
			conf:      &authModel.AuthMethodJWT{},
			header:    "Bearer " + badSignatureToken,
			jwkGetter: testJWKGetter,
			want:      false,
		},
		{
			name:      "required role missing",
			conf:      &authModel.AuthMethodJWT{Roles: []string{"super-admin"}},
			header:    "Bearer " + validToken,
			jwkGetter: testJWKGetter,
			want:      false,
		},
		{
			name:      "authorized",
			conf:      &authModel.AuthMethodJWT{Roles: []string{"moderator"}},
			header:    "Bearer " + validToken,
			jwkGetter: testJWKGetter,
			want:      true,
		},
		{
			name:      "authorized by keycloak resource_access roles",
			conf:      &authModel.AuthMethodJWT{Roles: []string{"admin"}},
			header:    "Bearer " + keycloakResourceAccessToken,
			jwkGetter: testJWKGetter,
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "http://localhost", nil)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}

			got := New(tt.jwkGetter, tt.conf).Authorize(req)
			require.Equal(t, tt.want, got)
		})
	}
}
