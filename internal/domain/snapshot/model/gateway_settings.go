package model

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
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
}

// Version changes whenever the settings do. It is keyed: the version is
// readable over the REST API, and a plain hash of a weak password could be
// guessed from it. Empty for empty settings.
func (m *GatewaySettings) Version(key string) string {
	if m.RedisAddr == "" {
		return ""
	}
	mac := hmac.New(sha256.New, []byte(key))
	for _, part := range []string{m.RedisAddr, m.RedisPassword, strconv.Itoa(m.RedisDB)} {
		mac.Write([]byte(part))
		mac.Write([]byte{0})
	}
	return hex.EncodeToString(mac.Sum(nil))[:16]
}
