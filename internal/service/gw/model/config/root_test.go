package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRootNormalize(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := Root{
			PublicBaseUrl: " https://public.example.com/ ",
			Timeout: RootTimeout{
				Global:     5 * time.Second,
				ReadHeader: 2 * time.Second,
				Read:       3 * time.Second,
			},
			Cors: RootCors{
				MaxAge:       "  1h ",
				AllowOrigins: []string{" https://a.example ", "", "https://b.example"},
				AllowMethods: []string{" GET ", "", "POST"},
				AllowHeaders: []string{" Authorization ", "", "Content-Type"},
			},
			Jwt: []*RootJwt{
				{
					JwkUrl:        " https://issuer.example/.well-known/jwks.json ",
					CacheDuration: 1 * time.Minute,
					RolesPath:     " roles.path ",
				},
			},
			Apps: []*App{
				{
					PublicPathPrefix: " /api ",
					Backend:          AppBackend{UrlStr: "https://backend.example"},
				},
			},
		}

		err := m.Normalize()
		require.NoError(t, err)
		require.Equal(t, "https://public.example.com", m.PublicBaseUrl)
		require.Equal(t, []string{"GET", "POST"}, m.Cors.AllowMethods)
	})

	t.Run("error empty public base url", func(t *testing.T) {
		m := Root{}
		err := m.Normalize()
		require.ErrorIs(t, err, errEmptyValue)
	})

	t.Run("error nested app normalize", func(t *testing.T) {
		m := Root{
			PublicBaseUrl: "https://public.example.com",
			Jwt: []*RootJwt{
				{
					JwkUrl:        "https://issuer.example/jwks",
					CacheDuration: time.Second,
					RolesPath:     "roles",
				},
			},
			Apps: []*App{
				{
					PublicPathPrefix: "api",
					Backend:          AppBackend{UrlStr: "://bad"},
				},
			},
		}
		err := m.Normalize()
		require.Error(t, err)
	})
}

func TestRootTimeoutNormalize(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := RootTimeout{
			Global:     1 * time.Second,
			ReadHeader: 2 * time.Second,
			Read:       3 * time.Second,
		}
		err := m.Normalize()
		require.NoError(t, err)
	})

	t.Run("error negative value", func(t *testing.T) {
		m := RootTimeout{Read: -1}
		err := m.Normalize()
		require.ErrorIs(t, err, errNegativeValue)
	})
}

func TestRootCorsNormalize(t *testing.T) {
	m := RootCors{
		MaxAge:       " 30m ",
		AllowOrigins: []string{" https://a.example ", "", "https://b.example"},
		AllowMethods: []string{" GET ", "", "POST "},
		AllowHeaders: []string{" Authorization ", "", "Content-Type "},
	}

	err := m.Normalize()
	require.NoError(t, err)
	require.Equal(t, "30m", m.MaxAge)
	require.Equal(t, []string{"https://a.example", "https://b.example"}, m.AllowOrigins)
	require.Equal(t, []string{"GET", "POST"}, m.AllowMethods)
	require.Equal(t, []string{"Authorization", "Content-Type"}, m.AllowHeaders)
}

func TestRootJwtNormalize(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := RootJwt{
			JwkUrl:        " https://issuer.example/jwks ",
			CacheDuration: 5 * time.Minute,
			RolesPath:     " roles ",
		}
		err := m.Normalize()
		require.NoError(t, err)
		require.Equal(t, "https://issuer.example/jwks", m.JwkUrl)
		require.Equal(t, "roles", m.RolesPath)
	})

	t.Run("error empty roles path", func(t *testing.T) {
		m := RootJwt{
			JwkUrl:        "https://issuer.example/jwks",
			CacheDuration: time.Minute,
			RolesPath:     " ",
		}
		err := m.Normalize()
		require.ErrorIs(t, err, errEmptyValue)
	})
}
