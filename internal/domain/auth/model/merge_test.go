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

	t.Run("basic single-type methods merge users on extend", func(t *testing.T) {
		root := Auth{
			Enabled: true,
			Mode:    "extend",
			Methods: []*AuthMethod{
				{Basic: &AuthMethodBasic{Users: []AuthMethodBasicUser{{Username: "u1", Password: "p1"}}}},
			},
		}
		app := Auth{
			Enabled: true,
			Mode:    "extend",
			Methods: []*AuthMethod{
				{Basic: &AuthMethodBasic{Users: []AuthMethodBasicUser{{Username: "u2", Password: "p2"}}}},
			},
		}

		final := Merge(root, app)

		require.Len(t, final.Methods, 1)
		require.NotNil(t, final.Methods[0].Basic)
		require.Equal(t, []AuthMethodBasicUser{
			{Username: "u1", Password: "p1"},
			{Username: "u2", Password: "p2"},
		}, final.Methods[0].Basic.Users)
	})

	t.Run("api key methods with different headers do not merge", func(t *testing.T) {
		root := Auth{
			Enabled: true,
			Mode:    "extend",
			Methods: []*AuthMethod{
				{APIKey: &AuthMethodAPIKey{Header: "X-Api-Key", Keys: []string{"k1"}}},
			},
		}
		app := Auth{
			Enabled: true,
			Mode:    "extend",
			Methods: []*AuthMethod{
				{APIKey: &AuthMethodAPIKey{Header: "X-Other-Key", Keys: []string{"k2"}}},
			},
		}

		final := Merge(root, app)

		require.Len(t, final.Methods, 2)
		require.NotNil(t, final.Methods[0].APIKey)
		require.NotNil(t, final.Methods[1].APIKey)
		require.Equal(t, "X-Api-Key", final.Methods[0].APIKey.Header)
		require.Equal(t, []string{"k1"}, final.Methods[0].APIKey.Keys)
		require.Equal(t, "X-Other-Key", final.Methods[1].APIKey.Header)
		require.Equal(t, []string{"k2"}, final.Methods[1].APIKey.Keys)
	})

	t.Run("api key methods merge after normalize sets default header", func(t *testing.T) {
		root := Auth{
			Enabled: true,
			Mode:    "extend",
			Methods: []*AuthMethod{
				{APIKey: &AuthMethodAPIKey{Header: "", Keys: []string{"k1"}}},
			},
		}
		app := Auth{
			Enabled: true,
			Mode:    "extend",
			Methods: []*AuthMethod{
				{APIKey: &AuthMethodAPIKey{Header: "X-Api-Key", Keys: []string{"k2"}}},
			},
		}
		require.NoError(t, root.Normalize())
		require.NoError(t, app.Normalize())

		final := Merge(root, app)

		require.Len(t, final.Methods, 1)
		require.NotNil(t, final.Methods[0].APIKey)
		require.Equal(t, "X-Api-Key", final.Methods[0].APIKey.Header)
		require.Equal(t, []string{"k1", "k2"}, final.Methods[0].APIKey.Keys)
	})

	t.Run("ip validation single-type methods merge allowed ips on extend", func(t *testing.T) {
		root := Auth{
			Enabled: true,
			Mode:    "extend",
			Methods: []*AuthMethod{
				{IPValidation: &AuthMethodIPValidation{AllowedIps: []string{"10.0.0.1"}}},
			},
		}
		app := Auth{
			Enabled: true,
			Mode:    "extend",
			Methods: []*AuthMethod{
				{IPValidation: &AuthMethodIPValidation{AllowedIps: []string{"10.0.0.2"}}},
			},
		}

		final := Merge(root, app)

		require.Len(t, final.Methods, 1)
		require.NotNil(t, final.Methods[0].IPValidation)
		require.Equal(t, []string{"10.0.0.1", "10.0.0.2"}, final.Methods[0].IPValidation.AllowedIps)
	})

	t.Run("single-type methods are not merged with mixed methods", func(t *testing.T) {
		root := Auth{
			Enabled: true,
			Mode:    "extend",
			Methods: []*AuthMethod{
				{Basic: &AuthMethodBasic{Users: []AuthMethodBasicUser{{Username: "u1", Password: "p1"}}}},
			},
		}
		app := Auth{
			Enabled: true,
			Mode:    "extend",
			Methods: []*AuthMethod{
				{
					Basic:  &AuthMethodBasic{Users: []AuthMethodBasicUser{{Username: "u2", Password: "p2"}}},
					APIKey: &AuthMethodAPIKey{Keys: []string{"k1"}},
				},
			},
		}

		final := Merge(root, app)

		require.Len(t, final.Methods, 2)
		require.NotNil(t, final.Methods[0].Basic)
		require.Equal(t, []AuthMethodBasicUser{{Username: "u1", Password: "p1"}}, final.Methods[0].Basic.Users)
		require.NotNil(t, final.Methods[1].Basic)
		require.Equal(t, []AuthMethodBasicUser{{Username: "u2", Password: "p2"}}, final.Methods[1].Basic.Users)
		require.NotNil(t, final.Methods[1].APIKey)
	})
}
