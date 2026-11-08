package jwk

import "crypto"

type PublicKeyGetter interface {
	// GetPublicKey returns the key kid of the named provider, or of any
	// provider when provider is empty, with its algorithm; nil when unknown.
	GetPublicKey(provider, kid string) (crypto.PublicKey, string)
}
