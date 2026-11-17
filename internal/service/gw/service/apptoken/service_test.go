package apptoken

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExchangeScopesAlwaysAskOpenid(t *testing.T) {
	assert.Equal(t, []string{"openid"}, exchangeScopes(nil))
	assert.Equal(t, []string{"openid", "roles"}, exchangeScopes([]string{"roles"}))
	assert.Equal(t, []string{"openid", "roles"}, exchangeScopes([]string{"roles", "openid"}), "not twice")
}
