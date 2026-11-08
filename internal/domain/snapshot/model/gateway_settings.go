package model

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"strconv"

	"golang.org/x/crypto/hkdf"
)

// CookieKeySize is the AES-256 key the gateways seal the refresh cookie with.
const CookieKeySize = 32

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

// Version changes whenever the settings do. It is keyed: the version is
// readable over the REST API, and a plain hash of a weak password could be
// guessed from it. Empty for empty settings.
func (m *GatewaySettings) Version(key string) string {
	if m.RedisAddr == "" && len(m.CookieKey) == 0 {
		return ""
	}
	mac := hmac.New(sha256.New, []byte(key))
	for _, part := range [][]byte{[]byte(m.RedisAddr), []byte(m.RedisPassword), []byte(strconv.Itoa(m.RedisDB)), m.CookieKey} {
		mac.Write(part)
		mac.Write([]byte{0})
	}
	return hex.EncodeToString(mac.Sum(nil))[:16]
}
