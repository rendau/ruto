package jwk

import "crypto"

// Item is one key of a JWK set (RFC 7517): an RSA key (n, e), an elliptic
// curve key (crv, x, y) or an Edwards curve key (crv, x).
type Item struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Alg string `json:"alg"`
	Use string `json:"use"`
	N   string `json:"n,omitempty"`
	E   string `json:"e,omitempty"`
	Crv string `json:"crv,omitempty"`
	X   string `json:"x,omitempty"`
	Y   string `json:"y,omitempty"`

	// PublicKey is the parsed key: *rsa.PublicKey, *ecdsa.PublicKey or
	// ed25519.PublicKey, whatever the signing library expects for Alg.
	PublicKey crypto.PublicKey `json:"-"`
}

type ItemSet struct {
	Keys []*Item `json:"keys"`
}
