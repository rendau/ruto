package jwt

import (
	"crypto/rand"
	"crypto/rsa"
	"net/http"
	"net/http/httptest"
	"testing"

	jwtv5 "github.com/golang-jwt/jwt/v5"

	authModel "github.com/rendau/ruto/internal/domain/auth/model"
)

func BenchmarkAuthorizeRS256(b *testing.B) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		b.Fatalf("rsa.GenerateKey() error: %v", err)
	}
	publicKey := privateKey.PublicKey

	kid := "kid-1"
	alg := jwtv5.SigningMethodRS256.Alg()

	tokenObj := jwtv5.NewWithClaims(jwtv5.SigningMethodRS256, jwtv5.MapClaims{
		"sub":   "u1",
		"roles": []string{"viewer", "admin"},
	})
	tokenObj.Header["kid"] = kid

	signedToken, err := tokenObj.SignedString(privateKey)
	if err != nil {
		b.Fatalf("SignedString() error: %v", err)
	}

	authorizer := New(&testJWKGetterT{
		keys: map[string]struct {
			publicKey *rsa.PublicKey
			alg       string
		}{
			kid: {
				publicKey: &publicKey,
				alg:       alg,
			},
		},
	}, &authModel.AuthMethodJWT{
		Kid:   kid,
		Roles: []string{"admin"},
	})

	req := httptest.NewRequest(http.MethodGet, "http://localhost", nil)
	req.Header.Set("Authorization", "Bearer "+signedToken)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if !authorizer.Authorize(req) {
			b.Fatalf("Authorize() = false")
		}
	}
}
