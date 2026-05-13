package jwt

import (
	"crypto"
	"crypto/rsa"
	"encoding/base64"
	"errors"
	"math/big"
	"strings"

	"github.com/rendau/ruto/internal/service/gw/jwk"
)

var errInvalidJWTKey = errors.New("invalid jwt key")

func verifySignature(parts []string, headerAlg string, keyItem *jwk.Item) bool {
	pubKey, err := jwkToRSAPublicKey(keyItem)
	if err != nil {
		return false
	}

	alg, hashType, ok := resolveAlg(headerAlg, keyItem.Alg)
	if !ok {
		return false
	}
	if keyItem.Alg != "" && keyItem.Alg != alg {
		return false
	}

	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return false
	}

	signingInput := parts[0] + "." + parts[1]
	h := hashType.New()
	_, _ = h.Write([]byte(signingInput))
	digest := h.Sum(nil)

	return rsa.VerifyPKCS1v15(pubKey, hashType, digest, signature) == nil
}

func jwkToRSAPublicKey(item *jwk.Item) (*rsa.PublicKey, error) {
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
		return nil, errInvalidJWTKey
	}

	return &rsa.PublicKey{N: n, E: e}, nil
}

func resolveAlg(headerAlg, keyAlg string) (string, crypto.Hash, bool) {
	alg := strings.TrimSpace(headerAlg)
	if alg == "" {
		alg = strings.TrimSpace(keyAlg)
	}

	switch alg {
	case "RS256":
		return alg, crypto.SHA256, true
	case "RS384":
		return alg, crypto.SHA384, true
	case "RS512":
		return alg, crypto.SHA512, true
	default:
		return "", 0, false
	}
}
