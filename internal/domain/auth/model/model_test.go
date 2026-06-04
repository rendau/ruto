package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAuthMethodJWTNormalize(t *testing.T) {
	t.Run("kid is required", func(t *testing.T) {
		item := AuthMethodJWT{}
		err := item.Normalize()
		require.Error(t, err)
		require.Contains(t, err.Error(), "kid: empty")
	})

	t.Run("trims and deduplicates roles", func(t *testing.T) {
		item := AuthMethodJWT{
			Kid: " kid-1 ",
			Roles: []string{
				" admin ",
				"admin",
				" ",
				"moderator",
			},
		}

		err := item.Normalize()
		require.NoError(t, err)
		require.Equal(t, "kid-1", item.Kid)
		require.Equal(t, []string{"admin", "moderator"}, item.Roles)
	})
}
