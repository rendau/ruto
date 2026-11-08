package constant

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestJWTKeyAlgorithm(t *testing.T) {
	assert.Equal(t, "RS512", JWTKeyAlgorithm("RSA", "", "RS512"), "a named alg stays")
	assert.Empty(t, JWTKeyAlgorithm("RSA", "", ""), "an RSA key without alg could sign with any RS*/PS*")
	assert.Equal(t, "ES256", JWTKeyAlgorithm("EC", "P-256", ""))
	assert.Equal(t, "ES384", JWTKeyAlgorithm("EC", "P-384", ""))
	assert.Equal(t, "ES512", JWTKeyAlgorithm("EC", "P-521", ""))
	assert.Equal(t, "EdDSA", JWTKeyAlgorithm("OKP", "Ed25519", ""))
	assert.Empty(t, JWTKeyAlgorithm("OKP", "X25519", ""), "an exchange curve does not sign")
	assert.Empty(t, JWTKeyAlgorithm("EC", "secp256k1", ""), "not a curve we verify")

	for _, alg := range SupportedJWTAlgorithms {
		assert.True(t, IsSupportedJWTAlgorithm(alg), alg)
	}
	assert.False(t, IsSupportedJWTAlgorithm("HS256"), "symmetric algorithms need the provider's secret")
	assert.False(t, IsSupportedJWTAlgorithm("none"))
}
