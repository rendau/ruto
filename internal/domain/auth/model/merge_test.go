package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAuthMerge(t *testing.T) {
	t.Run("app replace removes root methods", func(t *testing.T) {
		root := Auth{
			Enabled: true,
			Methods: []*AuthMethod{{Basic: &AuthMethodBasic{}}},
		}
		app := Auth{
			Enabled: true,
			Mode:    "replace",
			Methods: []*AuthMethod{{APIKey: &AuthMethodAPIKey{}}},
		}
		ep := Auth{
			Enabled: true,
			Mode:    "extend",
			Methods: []*AuthMethod{{JWT: &AuthMethodJWT{}}},
		}

		final := Merge(Merge(root, app), ep)

		require.Len(t, final.Methods, 2)
		require.Nil(t, final.Methods[0].Basic)
		require.NotNil(t, final.Methods[0].APIKey)
		require.NotNil(t, final.Methods[1].JWT)
	})

	t.Run("endpoint replace drops inherited methods", func(t *testing.T) {
		root := Auth{
			Enabled: true,
			Methods: []*AuthMethod{{Basic: &AuthMethodBasic{}}},
		}
		app := Auth{
			Enabled: true,
			Methods: []*AuthMethod{{APIKey: &AuthMethodAPIKey{}}},
		}
		ep := Auth{
			Enabled: true,
			Mode:    "replace",
			Methods: []*AuthMethod{{JWT: &AuthMethodJWT{}}},
		}

		final := Merge(Merge(root, app), ep)

		require.Len(t, final.Methods, 1)
		require.NotNil(t, final.Methods[0].JWT)
	})

	t.Run("jwt with same kid merges roles on extend", func(t *testing.T) {
		root := Auth{
			Enabled: true,
			Mode:    "extend",
			Methods: []*AuthMethod{{JWT: &AuthMethodJWT{Kid: "kid-1", Roles: []string{"admin"}}}},
		}
		app := Auth{
			Enabled: true,
			Mode:    "extend",
			Methods: []*AuthMethod{{JWT: &AuthMethodJWT{Kid: "kid-1", Roles: []string{"moderator"}}}},
		}
		ep := Auth{
			Enabled: true,
			Mode:    "extend",
		}

		final := Merge(Merge(root, app), ep)

		require.Len(t, final.Methods, 1)
		require.NotNil(t, final.Methods[0].JWT)
		require.Equal(t, "kid-1", final.Methods[0].JWT.Kid)
		require.ElementsMatch(t, []string{"admin", "moderator"}, final.Methods[0].JWT.Roles)
	})

	t.Run("jwt with same kid is not merged when child method has extra auth type", func(t *testing.T) {
		root := Auth{
			Enabled: true,
			Mode:    "extend",
			Methods: []*AuthMethod{{JWT: &AuthMethodJWT{Kid: "kid-1", Roles: []string{"admin"}}}},
		}
		app := Auth{
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

		final := Merge(Merge(root, app), ep)

		require.Len(t, final.Methods, 2)
		require.NotNil(t, final.Methods[0].JWT)
		require.Equal(t, "kid-1", final.Methods[0].JWT.Kid)
		require.ElementsMatch(t, []string{"admin"}, final.Methods[0].JWT.Roles)

		require.NotNil(t, final.Methods[1].JWT)
		require.Equal(t, "kid-1", final.Methods[1].JWT.Kid)
		require.ElementsMatch(t, []string{"moderator"}, final.Methods[1].JWT.Roles)
		require.NotNil(t, final.Methods[1].APIKey)
	})

	t.Run("jwt with same kid is not merged when parent method has extra auth type", func(t *testing.T) {
		root := Auth{
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
		app := Auth{
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

		final := Merge(Merge(root, app), ep)

		require.Len(t, final.Methods, 2)
		require.NotNil(t, final.Methods[0].JWT)
		require.Equal(t, "kid-1", final.Methods[0].JWT.Kid)
		require.ElementsMatch(t, []string{"admin"}, final.Methods[0].JWT.Roles)
		require.NotNil(t, final.Methods[0].Basic)

		require.NotNil(t, final.Methods[1].JWT)
		require.Equal(t, "kid-1", final.Methods[1].JWT.Kid)
		require.ElementsMatch(t, []string{"moderator"}, final.Methods[1].JWT.Roles)
		require.Nil(t, final.Methods[1].Basic)
	})

	t.Run("jwt with different kid does not merge", func(t *testing.T) {
		root := Auth{
			Enabled: true,
			Mode:    "extend",
			Methods: []*AuthMethod{{JWT: &AuthMethodJWT{Kid: "kid-1", Roles: []string{"admin"}}}},
		}
		app := Auth{
			Enabled: true,
			Mode:    "extend",
			Methods: []*AuthMethod{{JWT: &AuthMethodJWT{Kid: "kid-2", Roles: []string{"moderator"}}}},
		}
		ep := Auth{
			Enabled: true,
			Mode:    "extend",
		}

		final := Merge(Merge(root, app), ep)

		require.Len(t, final.Methods, 2)
		require.NotNil(t, final.Methods[0].JWT)
		require.NotNil(t, final.Methods[1].JWT)
		require.Equal(t, "kid-1", final.Methods[0].JWT.Kid)
		require.Equal(t, "kid-2", final.Methods[1].JWT.Kid)
	})
}
