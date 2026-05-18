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

	t.Run("jwt with same kid merges roles on extend", func(t *testing.T) {
		root := &Auth{
			Enabled: true,
			Mode:    "extend",
			Methods: []*AuthMethod{{JWT: &AuthMethodJWT{Kid: "kid-1", Roles: []string{"admin"}}}},
		}
		app := &Auth{
			Enabled: true,
			Mode:    "extend",
			Methods: []*AuthMethod{{JWT: &AuthMethodJWT{Kid: "kid-1", Roles: []string{"moderator"}}}},
		}
		ep := Auth{
			Enabled: true,
			Mode:    "extend",
		}

		ep.Merge(root, app)

		require.Len(t, ep.Methods, 1)
		require.NotNil(t, ep.Methods[0].JWT)
		require.Equal(t, "kid-1", ep.Methods[0].JWT.Kid)
		require.ElementsMatch(t, []string{"admin", "moderator"}, ep.Methods[0].JWT.Roles)
	})

	t.Run("jwt with same kid is not merged when child method has extra auth type", func(t *testing.T) {
		root := &Auth{
			Enabled: true,
			Mode:    "extend",
			Methods: []*AuthMethod{{JWT: &AuthMethodJWT{Kid: "kid-1", Roles: []string{"admin"}}}},
		}
		app := &Auth{
			Enabled: true,
			Mode:    "extend",
			Methods: []*AuthMethod{
				{
					JWT: &AuthMethodJWT{Kid: "kid-1", Roles: []string{"moderator"}},
					APIKey: &AuthMethodAPIKey{
						Header: "X-API-Key",
						Keys:   []string{"k-1"},
					},
				},
			},
		}
		ep := Auth{
			Enabled: true,
			Mode:    "extend",
		}

		ep.Merge(root, app)

		require.Len(t, ep.Methods, 2)
		require.NotNil(t, ep.Methods[0].JWT)
		require.Equal(t, "kid-1", ep.Methods[0].JWT.Kid)
		require.ElementsMatch(t, []string{"admin"}, ep.Methods[0].JWT.Roles)

		require.NotNil(t, ep.Methods[1].JWT)
		require.Equal(t, "kid-1", ep.Methods[1].JWT.Kid)
		require.ElementsMatch(t, []string{"moderator"}, ep.Methods[1].JWT.Roles)
		require.NotNil(t, ep.Methods[1].APIKey)
	})

	t.Run("jwt with same kid is not merged when parent method has extra auth type", func(t *testing.T) {
		root := &Auth{
			Enabled: true,
			Mode:    "extend",
			Methods: []*AuthMethod{
				{
					JWT: &AuthMethodJWT{Kid: "kid-1", Roles: []string{"admin"}},
					Basic: &AuthMethodBasic{
						Users: []AuthMethodBasicUser{{Username: "u1", Password: "p1"}},
					},
				},
			},
		}
		app := &Auth{
			Enabled: true,
			Mode:    "extend",
			Methods: []*AuthMethod{
				{JWT: &AuthMethodJWT{Kid: "kid-1", Roles: []string{"moderator"}}},
			},
		}
		ep := Auth{
			Enabled: true,
			Mode:    "extend",
		}

		ep.Merge(root, app)

		require.Len(t, ep.Methods, 2)
		require.NotNil(t, ep.Methods[0].JWT)
		require.Equal(t, "kid-1", ep.Methods[0].JWT.Kid)
		require.ElementsMatch(t, []string{"admin"}, ep.Methods[0].JWT.Roles)
		require.NotNil(t, ep.Methods[0].Basic)

		require.NotNil(t, ep.Methods[1].JWT)
		require.Equal(t, "kid-1", ep.Methods[1].JWT.Kid)
		require.ElementsMatch(t, []string{"moderator"}, ep.Methods[1].JWT.Roles)
		require.Nil(t, ep.Methods[1].Basic)
	})

	t.Run("jwt with different kid does not merge", func(t *testing.T) {
		root := &Auth{
			Enabled: true,
			Mode:    "extend",
			Methods: []*AuthMethod{{JWT: &AuthMethodJWT{Kid: "kid-1", Roles: []string{"admin"}}}},
		}
		app := &Auth{
			Enabled: true,
			Mode:    "extend",
			Methods: []*AuthMethod{{JWT: &AuthMethodJWT{Kid: "kid-2", Roles: []string{"moderator"}}}},
		}
		ep := Auth{
			Enabled: true,
			Mode:    "extend",
		}

		ep.Merge(root, app)

		require.Len(t, ep.Methods, 2)
		require.NotNil(t, ep.Methods[0].JWT)
		require.NotNil(t, ep.Methods[1].JWT)
		require.Equal(t, "kid-1", ep.Methods[0].JWT.Kid)
		require.Equal(t, "kid-2", ep.Methods[1].JWT.Kid)
	})
}

func TestAuthMethodJWTNormalize(t *testing.T) {
	t.Run("kid is required", func(t *testing.T) {
		item := &AuthMethodJWT{}
		err := item.Normalize()
		require.Error(t, err)
		require.Contains(t, err.Error(), "kid: empty")
	})

	t.Run("trims and deduplicates roles", func(t *testing.T) {
		item := &AuthMethodJWT{
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
