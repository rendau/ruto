package jwk

import (
	"crypto/rsa"
)

type PublicKeyGetter interface {
	GetPublicKey(kid string) (*rsa.PublicKey, string)
}
