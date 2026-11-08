package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGatewaySettingsVersion(t *testing.T) {
	empty := GatewaySettings{}
	assert.Empty(t, empty.Version("k"))

	a := GatewaySettings{RedisAddr: "redis:6379", RedisPassword: "p", RedisDB: 1, CookieKey: CookieKeyFrom("s")}
	same := a
	assert.Equal(t, a.Version("k"), same.Version("k"))
	assert.Len(t, a.Version("k"), 16)

	for _, changed := range []GatewaySettings{
		{RedisAddr: "redis:6380", RedisPassword: "p", RedisDB: 1, CookieKey: CookieKeyFrom("s")},
		{RedisAddr: "redis:6379", RedisPassword: "q", RedisDB: 1, CookieKey: CookieKeyFrom("s")},
		{RedisAddr: "redis:6379", RedisPassword: "p", RedisDB: 2, CookieKey: CookieKeyFrom("s")},
		{RedisAddr: "redis:6379", RedisPassword: "p", RedisDB: 1, CookieKey: CookieKeyFrom("t")},
		{RedisAddr: "redis:6379", RedisPassword: "p", RedisDB: 1},
	} {
		assert.NotEqual(t, a.Version("k"), changed.Version("k"), changed)
	}
	assert.NotEqual(t, a.Version("k"), a.Version("other key"), "the version depends on the key")

	onlyKey := GatewaySettings{CookieKey: CookieKeyFrom("s")}
	assert.NotEmpty(t, onlyKey.Version("k"), "a cookie key alone is a setting")
}

func TestCookieKeyFrom(t *testing.T) {
	assert.Nil(t, CookieKeyFrom(""))
	assert.Len(t, CookieKeyFrom("x"), CookieKeySize)
	assert.Equal(t, CookieKeyFrom("secret"), CookieKeyFrom("secret"), "deterministic: every pod and restart gets the same key")
	assert.NotEqual(t, CookieKeyFrom("secret"), CookieKeyFrom("secret2"))
	assert.NotEqual(t, []byte("0123456789abcdef0123456789abcdef"), CookieKeyFrom("0123456789abcdef0123456789abcdef"), "derived, not used as is")
}
