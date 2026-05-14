package jwk

import (
	"crypto/rsa"
	"encoding/base64"
	"fmt"
	"math/big"
)

func toRSAPublicKey(item *Item) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(item.N)
	if err != nil {
		return nil, err
	}

	eBytes, err := base64.RawURLEncoding.DecodeString(item.E)
	if err != nil {
		return nil, err
	}

	n := new(big.Int).SetBytes(nBytes)
	e := int(new(big.Int).SetBytes(eBytes).Int64())
	if n.Sign() <= 0 || e <= 0 {
		return nil, fmt.Errorf("invalid key")
	}

	return &rsa.PublicKey{N: n, E: e}, nil
}
