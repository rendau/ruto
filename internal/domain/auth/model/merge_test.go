package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAuthMerge(t *testing.T) {
	t.Run("app replace removes root methods", func(t *testing.T) {
		root := &Auth{
			Enabled: true,
			Methods: []*AuthMethod{{Basic: &AuthMethodBasic{}}},
		}
		app := &Auth{
			Enabled: true,
			Mode:    "replace",
			Methods: []*AuthMethod{{APIKey: &AuthMethodAPIKey{}}},
		}
		ep := Auth{
			Enabled: true,
			Mode:    "extend",
			Methods: []*AuthMethod{{JWT: &AuthMethodJWT{}}},
		}

		ep.Merge(root, app)

		require.Len(t, ep.Methods, 2)
		require.Nil(t, ep.Methods[0].Basic)
		require.NotNil(t, ep.Methods[0].APIKey)
		require.NotNil(t, ep.Methods[1].JWT)
	})

	t.Run("endpoint replace drops inherited methods", func(t *testing.T) {
		root := &Auth{
			Enabled: true,
			Methods: []*AuthMethod{{Basic: &AuthMethodBasic{}}},
		}
		app := &Auth{
			Enabled: true,
			Methods: []*AuthMethod{{APIKey: &AuthMethodAPIKey{}}},
		}
		ep := Auth{
			Enabled: true,
			Mode:    "replace",
			Methods: []*AuthMethod{{JWT: &AuthMethodJWT{}}},
		}

		ep.Merge(root, app)

		require.Len(t, ep.Methods, 1)
		require.NotNil(t, ep.Methods[0].JWT)
	})
}
