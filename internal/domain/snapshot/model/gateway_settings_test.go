package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGatewaySettingsVersion(t *testing.T) {
	empty := GatewaySettings{}
	assert.Empty(t, empty.Version("k"))

	a := GatewaySettings{RedisAddr: "redis:6379", RedisPassword: "p", RedisDB: 1}
	same := a
	assert.Equal(t, a.Version("k"), same.Version("k"))
	assert.Len(t, a.Version("k"), 16)

	for _, changed := range []GatewaySettings{
		{RedisAddr: "redis:6380", RedisPassword: "p", RedisDB: 1},
		{RedisAddr: "redis:6379", RedisPassword: "q", RedisDB: 1},
		{RedisAddr: "redis:6379", RedisPassword: "p", RedisDB: 2},
	} {
		assert.NotEqual(t, a.Version("k"), changed.Version("k"), changed)
	}
	assert.NotEqual(t, a.Version("k"), a.Version("other key"), "the version depends on the key")
}
