package jwt

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"net/http"
	"testing"

	jwtv5 "github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"

	authModel "github.com/rendau/ruto/internal/domain/auth/model"
	"github.com/rendau/ruto/internal/service/gw/service/auth/model"
	"github.com/rendau/ruto/internal/service/gw/service/jwk"
)

type testJWKGetterT struct {
	provider string // name of the provider the keys belong to
	keys     map[string]struct {
		publicKey crypto.PublicKey
		alg       string
	}
}

func (s *testJWKGetterT) GetPublicKey(provider, kid string) (crypto.PublicKey, string) {
	if provider != "" && provider != s.provider {
		return nil, ""
	}
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
	validTokenWithoutRoles := signToken(
		jwtv5.SigningMethodRS256,
		itemKid,
		jwtv5.MapClaims{"sub": "u1"},
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

	keycloakRealmAccessToken := signToken(
		jwtv5.SigningMethodRS256,
		itemKid,
		jwtv5.MapClaims{
			"sub":          "u1",
			"realm_access": map[string]any{"roles": []any{"auditor"}},
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
		provider: "corp",
		keys: map[string]struct {
			publicKey crypto.PublicKey
			alg       string
		}{
			itemKid: {
				publicKey: &publicKey,
				alg:       itemAlg,
			},
		},
	}

	tests := []struct {
		name       string
		conf       *authModel.AuthMethodJWT
		token      string
		cookie     string
		jwkGetter  jwk.PublicKeyGetter
		want       model.Result
		fromCookie bool // the accepted token came from the cookie
	}{
		{
			name:       "token from the provider cookie",
			conf:       &authModel.AuthMethodJWT{Provider: "corp"},
			cookie:     "other=1; ruto_corp=" + validToken,
			jwkGetter:  testJWKGetter,
			want:       model.Allowed,
			fromCookie: true,
		},
		{
			name:      "cookie of another provider is not read",
			conf:      &authModel.AuthMethodJWT{Provider: "corp"},
			cookie:    "ruto_clients=" + validToken,
			jwkGetter: testJWKGetter,
			want:      model.NoCredentials,
		},
		{
			name:      "method without provider reads no cookie",
			conf:      &authModel.AuthMethodJWT{Kid: itemKid},
			cookie:    "ruto_corp=" + validToken,
			jwkGetter: testJWKGetter,
			want:      model.NoCredentials,
		},
		{
			name:      "authorization header wins over the cookie",
			conf:      &authModel.AuthMethodJWT{Provider: "corp"},
			token:     "Bearer " + badSignatureToken,
			cookie:    "ruto_corp=" + validToken,
			jwkGetter: testJWKGetter,
			want:      model.InvalidCredentials,
		},
		{
			name:      "cookie token without the required role",
			conf:      &authModel.AuthMethodJWT{Provider: "corp", Roles: []string{"super-admin"}},
			cookie:    "ruto_corp=" + validToken,
			jwkGetter: testJWKGetter,
			want:      model.DeniedAuthenticated,
		},
		{
			name:      "client role: found at that client",
			conf:      &authModel.AuthMethodJWT{Provider: "corp", Roles: []string{"ruto:admin"}},
			token:     "Bearer " + keycloakResourceAccessToken,
			jwkGetter: testJWKGetter,
			want:      model.Allowed,
		},
		{
			name:      "client role: the same role at another client does not count",
			conf:      &authModel.AuthMethodJWT{Provider: "corp", Roles: []string{"shop:admin"}},
			token:     "Bearer " + keycloakResourceAccessToken,
			jwkGetter: testJWKGetter,
			want:      model.DeniedAuthenticated,
		},
		{
			name:      "client wildcard: any role of that client",
			conf:      &authModel.AuthMethodJWT{Provider: "corp", Roles: []string{"ruto:*"}},
			token:     "Bearer " + keycloakResourceAccessToken,
			jwkGetter: testJWKGetter,
			want:      model.Allowed,
		},
		{
			name:      "client wildcard: no roles at that client",
			conf:      &authModel.AuthMethodJWT{Provider: "corp", Roles: []string{"shop:*"}},
			token:     "Bearer " + keycloakResourceAccessToken,
			jwkGetter: testJWKGetter,
			want:      model.DeniedAuthenticated,
		},
		{
			name:      "client wildcard is not satisfied by a realm role",
			conf:      &authModel.AuthMethodJWT{Provider: "corp", Roles: []string{"ruto:*"}},
			token:     "Bearer " + keycloakRealmAccessToken,
			jwkGetter: testJWKGetter,
			want:      model.DeniedAuthenticated,
		},
		{
			name:      "bare role still matches any client's roles",
			conf:      &authModel.AuthMethodJWT{Provider: "corp", Roles: []string{"admin"}},
			token:     "Bearer " + keycloakResourceAccessToken,
			jwkGetter: testJWKGetter,
			want:      model.Allowed,
		},
		{
			name:      "realm role",
			conf:      &authModel.AuthMethodJWT{Provider: "corp", Roles: []string{"auditor"}},
			token:     "Bearer " + keycloakRealmAccessToken,
			jwkGetter: testJWKGetter,
			want:      model.Allowed,
		},
		{
			name:      "client role is not satisfied by a realm role",
			conf:      &authModel.AuthMethodJWT{Provider: "corp", Roles: []string{"ruto:auditor"}},
			token:     "Bearer " + keycloakRealmAccessToken,
			jwkGetter: testJWKGetter,
			want:      model.DeniedAuthenticated,
		},
		{
			name:      "missing token",
			conf:      &authModel.AuthMethodJWT{Kid: itemKid},
			token:     "",
			jwkGetter: testJWKGetter,
			want:      model.NoCredentials,
		},
		{
			name:      "invalid algorithm",
			conf:      &authModel.AuthMethodJWT{Kid: itemKid},
			token:     "Bearer " + hsToken,
			jwkGetter: testJWKGetter,
			want:      model.InvalidCredentials,
		},
		{
			name:      "missing kid",
			conf:      &authModel.AuthMethodJWT{Kid: itemKid},
			token:     "Bearer " + missingKidToken,
			jwkGetter: testJWKGetter,
			want:      model.InvalidCredentials,
		},
		{
			name:      "kid is not allowed by config",
			conf:      &authModel.AuthMethodJWT{Kid: "kid-2"},
			token:     "Bearer " + validToken,
			jwkGetter: testJWKGetter,
			want:      model.InvalidCredentials,
		},
		{
			name:      "kid not found in jwk service",
			conf:      &authModel.AuthMethodJWT{Kid: itemKid},
			token:     "Bearer " + validToken,
			jwkGetter: &testJWKGetterT{},
			want:      model.InvalidCredentials,
		},
		{
			name:      "any kid of the provider when kid is not pinned",
			conf:      &authModel.AuthMethodJWT{Provider: "corp"},
			token:     "Bearer " + validToken,
			jwkGetter: testJWKGetter,
			want:      model.Allowed,
		},
		{
			name:      "key of another provider is rejected",
			conf:      &authModel.AuthMethodJWT{Provider: "clients"},
			token:     "Bearer " + validToken,
			jwkGetter: testJWKGetter,
			want:      model.InvalidCredentials,
		},
		{
			name:      "provider and pinned kid together",
			conf:      &authModel.AuthMethodJWT{Provider: "corp", Kid: "kid-2"},
			token:     "Bearer " + validToken,
			jwkGetter: testJWKGetter,
			want:      model.InvalidCredentials,
		},
		{
			name:  "jwk alg mismatch",
			conf:  &authModel.AuthMethodJWT{Kid: itemKid},
			token: "Bearer " + validToken,
			jwkGetter: &testJWKGetterT{
				keys: map[string]struct {
					publicKey crypto.PublicKey
					alg       string
				}{
					itemKid: {
						publicKey: &publicKey,
						alg:       jwtv5.SigningMethodRS384.Alg(),
					},
				},
			},
			want: model.InvalidCredentials,
		},
		{
			name:      "invalid signature",
			conf:      &authModel.AuthMethodJWT{Kid: itemKid},
			token:     "Bearer " + badSignatureToken,
			jwkGetter: testJWKGetter,
			want:      model.InvalidCredentials,
		},
		{
			name:      "required role missing",
			conf:      &authModel.AuthMethodJWT{Kid: itemKid, Roles: []string{"super-admin"}},
			token:     "Bearer " + validToken,
			jwkGetter: testJWKGetter,
			want:      model.DeniedAuthenticated,
		},
		{
			name:      "authorized",
			conf:      &authModel.AuthMethodJWT{Kid: itemKid, Roles: []string{"moderator"}},
			token:     "Bearer " + validToken,
			jwkGetter: testJWKGetter,
			want:      model.Allowed,
		},
		{
			name:      "authorized when role is not required and token has no roles claims",
			conf:      &authModel.AuthMethodJWT{Kid: itemKid},
			token:     "Bearer " + validTokenWithoutRoles,
			jwkGetter: testJWKGetter,
			want:      model.Allowed,
		},
		{
			name:      "authorized by keycloak resource_access roles",
			conf:      &authModel.AuthMethodJWT{Kid: itemKid, Roles: []string{"admin"}},
			token:     "Bearer " + keycloakResourceAccessToken,
			jwkGetter: testJWKGetter,
			want:      model.Allowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headers := http.Header{}
			headers.Set("Authorization", tt.token)
			if tt.cookie != "" {
				headers.Set("Cookie", tt.cookie)
			}

			req := &model.AuthRequest{Headers: headers}
			got := New(tt.conf, tt.jwkGetter).Authorize(req)
			require.Equal(t, tt.want, got)
			if tt.fromCookie {
				require.Equal(t, validToken, req.CookieToken())
			} else {
				require.Empty(t, req.CookieToken())
			}
		})
	}
}
