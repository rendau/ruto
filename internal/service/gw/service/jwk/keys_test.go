package jwk

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"math/big"
	"testing"

	jwtv5 "github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func b64(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }

func rsaItem(t *testing.T, alg string) (*Item, crypto.Signer) {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	return &Item{Kty: "RSA", Alg: alg, Kid: alg, N: b64(priv.N.Bytes()), E: b64(big.NewInt(int64(priv.E)).Bytes())}, priv
}

func ecItem(t *testing.T, curve elliptic.Curve, crv, alg string) (*Item, crypto.Signer) {
	t.Helper()
	priv, err := ecdsa.GenerateKey(curve, rand.Reader)
	require.NoError(t, err)
	size := (curve.Params().BitSize + 7) / 8
	return &Item{Kty: "EC", Alg: alg, Kid: alg, Crv: crv,
		X: b64(priv.X.FillBytes(make([]byte, size))), Y: b64(priv.Y.FillBytes(make([]byte, size)))}, priv
}

func edItem(t *testing.T) (*Item, crypto.Signer) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	return &Item{Kty: "OKP", Alg: "EdDSA", Kid: "EdDSA", Crv: "Ed25519", X: b64(pub)}, priv
}

// TestToPublicKeyVerifiesTokens parses a key of every supported kind and
// checks that a token signed with the matching private key verifies with
// what the jwt library gets from GetPublicKey.
func TestToPublicKeyVerifiesTokens(t *testing.T) {
	rs256, rs256Priv := rsaItem(t, "RS256")
	ps256, ps256Priv := rsaItem(t, "PS256")
	es256, es256Priv := ecItem(t, elliptic.P256(), "P-256", "ES256")
	es384, es384Priv := ecItem(t, elliptic.P384(), "P-384", "ES384")
	es512, es512Priv := ecItem(t, elliptic.P521(), "P-521", "ES512")
	eddsa, eddsaPriv := edItem(t)

	for _, tc := range []struct {
		item *Item
		priv crypto.Signer
	}{
		{rs256, rs256Priv}, {ps256, ps256Priv}, {es256, es256Priv}, {es384, es384Priv}, {es512, es512Priv}, {eddsa, eddsaPriv},
	} {
		t.Run(tc.item.Alg, func(t *testing.T) {
			key, err := toPublicKey(tc.item)
			require.NoError(t, err)

			method := jwtv5.GetSigningMethod(tc.item.Alg)
			require.NotNil(t, method)
			token, err := jwtv5.NewWithClaims(method, jwtv5.MapClaims{"sub": "u1"}).SignedString(tc.priv)
			require.NoError(t, err)

			parsed, err := jwtv5.Parse(token, func(*jwtv5.Token) (any, error) { return key, nil })
			require.NoError(t, err)
			assert.True(t, parsed.Valid)
		})
	}
}

func TestToPublicKeyRejectsMismatches(t *testing.T) {
	es256, _ := ecItem(t, elliptic.P256(), "P-256", "ES256")
	rs256, _ := rsaItem(t, "RS256")
	eddsa, _ := edItem(t)

	wrongAlgEC := *es256
	wrongAlgEC.Alg = "ES384"
	wrongCurve := *es256
	wrongCurve.Crv = "P-384"
	rsaAsEC := *rs256
	rsaAsEC.Alg = "ES256"
	shortEd := *eddsa
	shortEd.X = b64([]byte("short"))
	unknownKty := *rs256
	unknownKty.Kty = "oct"
	offCurve := *es256
	offCurve.Y = es256.X

	for name, item := range map[string]*Item{
		"EC key announced for another curve's alg": &wrongAlgEC,
		"EC point of another curve":                &wrongCurve,
		"RSA key announced as ES256":               &rsaAsEC,
		"Ed25519 key of a wrong size":              &shortEd,
		"symmetric key":                            &unknownKty,
		"point not on the curve":                   &offCurve,
	} {
		_, err := toPublicKey(item)
		assert.Error(t, err, name)
	}
}
