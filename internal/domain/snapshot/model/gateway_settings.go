package model

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"strconv"

	"golang.org/x/crypto/hkdf"
)

const (
	// CookieKeySize is the AES-256 key the gateways seal the refresh cookie with.
	CookieKeySize = 32
	// SigningKeySize is the P-256 scalar of the key the gateways sign ruto's
	// own tokens with (ES256).
	SigningKeySize = 32
)

// GatewaySettings are runtime settings core hands to the gateways along with
// the snapshot. They come from core's environment: they are not part of the
// published config, not stored and not shown in the admin.
type GatewaySettings struct {
	// Redis the gateways share login session refreshes through, so that all
	// pods together make one call to the provider per refresh token. An empty
	// address: each gateway refreshes on its own.
	RedisAddr     string
	RedisPassword string
	RedisDB       int
	// CookieKey seals the refresh cookie (refresh and id token) of a login
	// session, the same on every pod. Empty: the gateways issue no refresh
	// cookie, sessions cannot be renewed.
	CookieKey []byte
	// SigningKey signs ruto's own tokens, the gateway being an OpenID Connect
	// broker for the fronts: the raw P-256 scalar, the same on every pod.
	// Empty: the gateways publish no keys and issue no tokens.
	SigningKey []byte
}

// CookieKeyFrom derives the cookie key from a secret (HKDF-SHA256), so one
// secret of any length and strength yields a key of the right size and the
// secret itself never leaves core. Empty secret: no key.
func CookieKeyFrom(secret string) []byte {
	if secret == "" {
		return nil
	}
	key := make([]byte, CookieKeySize)
	if _, err := io.ReadFull(hkdf.New(sha256.New, []byte(secret), nil, []byte("ruto gateway refresh cookie")), key); err != nil {
		panic(err) // hkdf fails only past its output limit, far above one key
	}
	return key
}

// SigningKeyFrom derives the signing key from a secret, like CookieKeyFrom but
// under its own label, so the two keys are unrelated. Empty secret: no key.
func SigningKeyFrom(secret string) []byte {
	if secret == "" {
		return nil
	}
	r := hkdf.New(sha256.New, []byte(secret), nil, []byte("ruto gateway signing key"))
	key := make([]byte, SigningKeySize)
	// A scalar must be below the curve order; a random 32 bytes are not one
	// time in about 2^32, then the next bytes of the stream are taken.
	for {
		if _, err := io.ReadFull(r, key); err != nil {
			panic(err) // hkdf fails only past its output limit, far above a few keys
		}
		if _, err := ecdsa.ParseRawPrivateKey(elliptic.P256(), key); err == nil {
			return key
		}
	}
}

// Version changes whenever the settings do. It is keyed: the version is
// readable over the REST API, and a plain hash of a weak password could be
// guessed from it. Empty for empty settings.
func (m *GatewaySettings) Version(key string) string {
	if m.RedisAddr == "" && len(m.CookieKey) == 0 && len(m.SigningKey) == 0 {
		return ""
	}
	mac := hmac.New(sha256.New, []byte(key))
	for _, part := range [][]byte{[]byte(m.RedisAddr), []byte(m.RedisPassword), []byte(strconv.Itoa(m.RedisDB)), m.CookieKey, m.SigningKey} {
		mac.Write(part)
		mac.Write([]byte{0})
	}
	return hex.EncodeToString(mac.Sum(nil))[:16]
}
