package jwt

import (
	"crypto/rsa"
)

type JwkGetterI interface {
	GetPublicKey(kid string) (*rsa.PublicKey, string)
}
