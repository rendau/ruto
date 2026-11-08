package jwk

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rsa"
	"encoding/base64"
	"fmt"
	"math/big"
)

// curves maps an EC key's crv to the curve and the one algorithm it signs
// with (RFC 7518 §3.4).
var curves = map[string]struct {
	curve elliptic.Curve
	alg   string
}{
	"P-256": {elliptic.P256(), "ES256"},
	"P-384": {elliptic.P384(), "ES384"},
	"P-521": {elliptic.P521(), "ES512"},
}

// toPublicKey parses the key material by its type and checks that it fits
// the algorithm the key is announced for.
func toPublicKey(item *Item) (crypto.PublicKey, error) {
	switch item.Kty {
	case "RSA":
		return toRSAPublicKey(item)
	case "EC":
		return toECPublicKey(item)
	case "OKP":
		return toEdPublicKey(item)
	default:
		return nil, fmt.Errorf("unsupported kty %q", item.Kty)
	}
}

func toRSAPublicKey(item *Item) (*rsa.PublicKey, error) {
	switch item.Alg {
	case "RS256", "RS384", "RS512", "PS256", "PS384", "PS512":
	default:
		return nil, fmt.Errorf("alg %q is not an RSA algorithm", item.Alg)
	}

	n, err := decodeBigInt(item.N)
	if err != nil {
		return nil, fmt.Errorf("n: %w", err)
	}
	e, err := decodeBigInt(item.E)
	if err != nil {
		return nil, fmt.Errorf("e: %w", err)
	}
	if n.Sign() <= 0 || e.Sign() <= 0 || !e.IsInt64() {
		return nil, fmt.Errorf("invalid key")
	}

	return &rsa.PublicKey{N: n, E: int(e.Int64())}, nil
}

func toECPublicKey(item *Item) (*ecdsa.PublicKey, error) {
	c, ok := curves[item.Crv]
	if !ok {
		return nil, fmt.Errorf("unsupported crv %q", item.Crv)
	}
	if item.Alg != c.alg {
		return nil, fmt.Errorf("alg %q does not fit crv %q", item.Alg, item.Crv)
	}

	x, err := decodeBigInt(item.X)
	if err != nil {
		return nil, fmt.Errorf("x: %w", err)
	}
	y, err := decodeBigInt(item.Y)
	if err != nil {
		return nil, fmt.Errorf("y: %w", err)
	}
	if !c.curve.IsOnCurve(x, y) {
		return nil, fmt.Errorf("point is not on %s", item.Crv)
	}

	return &ecdsa.PublicKey{Curve: c.curve, X: x, Y: y}, nil
}

func toEdPublicKey(item *Item) (ed25519.PublicKey, error) {
	if item.Crv != "Ed25519" {
		return nil, fmt.Errorf("unsupported crv %q", item.Crv)
	}
	if item.Alg != "EdDSA" {
		return nil, fmt.Errorf("alg %q does not fit crv Ed25519", item.Alg)
	}

	x, err := base64.RawURLEncoding.DecodeString(item.X)
	if err != nil {
		return nil, fmt.Errorf("x: %w", err)
	}
	if len(x) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("x: %d bytes, want %d", len(x), ed25519.PublicKeySize)
	}

	return ed25519.PublicKey(x), nil
}

func decodeBigInt(raw string) (*big.Int, error) {
	if raw == "" {
		return nil, fmt.Errorf("empty")
	}
	bytes, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return nil, err
	}
	return new(big.Int).SetBytes(bytes), nil
}
