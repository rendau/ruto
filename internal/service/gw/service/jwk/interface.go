package jwk

import (
	"crypto/rsa"
)

type PublicKeyGetter interface {
	// GetPublicKey returns the key kid of the named provider, or of any
	// provider when provider is empty; nil when unknown.
	GetPublicKey(provider, kid string) (*rsa.PublicKey, string)
}
