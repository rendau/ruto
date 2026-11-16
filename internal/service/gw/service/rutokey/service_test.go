package rutokey

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	snapshotModel "github.com/rendau/ruto/internal/domain/snapshot/model"
)

// publicKeys parses a JWK set the way a front's library does: by the members
// of the standard, knowing nothing about how the gateway made it.
func publicKeys(t *testing.T, jwks []byte) map[string]*ecdsa.PublicKey {
	var set struct {
		Keys []jwkItem `json:"keys"`
	}
	require.NoError(t, json.Unmarshal(jwks, &set))
	result := map[string]*ecdsa.PublicKey{}
	for _, item := range set.Keys {
		require.Equal(t, "EC", item.Kty)
		require.Equal(t, "P-256", item.Crv)
		require.Equal(t, "ES256", item.Alg)
		require.Equal(t, "sig", item.Use)
		x, err := base64.RawURLEncoding.DecodeString(item.X)
		require.NoError(t, err)
		y, err := base64.RawURLEncoding.DecodeString(item.Y)
		require.NoError(t, err)
		key, err := ecdsa.ParseUncompressedPublicKey(elliptic.P256(), append(append([]byte{4}, x...), y...))
		require.NoError(t, err)
		result[item.Kid] = key
	}
	return result
}

func TestSignVerifiesByJWKS(t *testing.T) {
	s := New()
	s.Set(snapshotModel.SigningKeyFrom("secret"))

	jwks, ok := s.JWKS()
	require.True(t, ok)
	keys := publicKeys(t, jwks)
	require.Len(t, keys, 1)
	assert.Equal(t, s.Kids(), []string{thumbprintOf(keys)})

	signed, err := s.Sign(jwt.MapClaims{"sub": "u-1", "exp": time.Now().Add(time.Minute).Unix()})
	require.NoError(t, err)

	parsed, err := jwt.Parse(signed, func(token *jwt.Token) (any, error) {
		return keys[token.Header["kid"].(string)], nil
	}, jwt.WithValidMethods([]string{"ES256"}))
	require.NoError(t, err)
	assert.Equal(t, "u-1", parsed.Claims.(jwt.MapClaims)["sub"])
}

func thumbprintOf(keys map[string]*ecdsa.PublicKey) string {
	for kid := range keys {
		return kid
	}
	return ""
}

func TestSameKeyOnEveryPod(t *testing.T) {
	a, b, other := New(), New(), New()
	a.Set(snapshotModel.SigningKeyFrom("secret"))
	b.Set(snapshotModel.SigningKeyFrom("secret"))
	other.Set(snapshotModel.SigningKeyFrom("another secret"))

	jwksA, _ := a.JWKS()
	jwksB, _ := b.JWKS()
	assert.Equal(t, jwksA, jwksB)
	assert.Equal(t, a.Kids(), b.Kids())
	assert.NotEqual(t, a.Kids(), other.Kids(), "a new secret is a new key with a new kid")
	assert.Len(t, a.Kids()[0], 43, "base64url of a sha256")
}

func TestNoKey(t *testing.T) {
	s := New()
	_, ok := s.JWKS()
	assert.False(t, ok)
	assert.Empty(t, s.Kids())
	_, err := s.Sign(jwt.MapClaims{})
	assert.ErrorIs(t, err, ErrNoKey)

	s.Set(snapshotModel.SigningKeyFrom("secret"))
	require.NotEmpty(t, s.Kids())
	s.Set([]byte("short"))
	assert.Empty(t, s.Kids(), "a broken key drops the previous one")
	s.Set(snapshotModel.SigningKeyFrom("secret"))
	s.Set(nil)
	assert.Empty(t, s.Kids())
}
