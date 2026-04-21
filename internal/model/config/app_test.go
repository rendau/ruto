package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAppNormalize(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := App{
			Id:               "  app-id  ",
			PublicPathPrefix: " /public ",
			Backend: AppBackend{
				UrlStr: " https://example.com/base ",
			},
			Endpoints: []*Endpoint{
				{
					Method: " get ",
					Path:   " /v1/ping ",
					Backend: EndpointBackend{
						CustomPath: " /ping ",
					},
				},
			},
		}

		err := m.Normalize()
		require.NoError(t, err)
		require.Equal(t, "app-id", m.Id)
		require.Equal(t, "/public", m.PublicPathPrefix)
		require.NotNil(t, m.Backend.Url)
		require.Equal(t, "https://example.com/base", m.Backend.Url.String())
		require.Equal(t, "GET", m.Endpoints[0].Method)
	})

	t.Run("error empty public path prefix", func(t *testing.T) {
		m := App{
			PublicPathPrefix: " / ",
			Backend:          AppBackend{UrlStr: "https://example.com"},
		}
		err := m.Normalize()
		require.ErrorIs(t, err, errEmptyValue)
	})

	t.Run("error endpoint normalize", func(t *testing.T) {
		m := App{
			PublicPathPrefix: "public",
			Backend:          AppBackend{UrlStr: "https://example.com"},
			Endpoints: []*Endpoint{
				{
					Method: " ",
					Path:   "/ok",
					Backend: EndpointBackend{
						CustomPath: "/ok",
					},
				},
			},
		}
		err := m.Normalize()
		require.ErrorIs(t, err, errEmptyValue)
	})
}

func TestAppBackendNormalize(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := AppBackend{UrlStr: " https://example.com/api "}
		err := m.Normalize()
		require.NoError(t, err)
		require.Equal(t, "https://example.com/api", m.UrlStr)
		require.NotNil(t, m.Url)
		require.Equal(t, "https://example.com/api", m.Url.String())
	})

	t.Run("error invalid url", func(t *testing.T) {
		m := AppBackend{UrlStr: "://bad"}
		err := m.Normalize()
		require.Error(t, err)
	})

	t.Run("error empty url", func(t *testing.T) {
		m := AppBackend{UrlStr: "  "}
		err := m.Normalize()
		require.ErrorIs(t, err, errEmptyValue)
	})
}
