package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEndpointNormalize(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := Endpoint{
			Id:     "  endpoint-id  ",
			Method: " post ",
			Path:   " /v1/items ",
			Backend: EndpointBackend{
				CustomPath: " /api/items ",
			},
			JwtValidation: EndpointJwtValidation{
				Roles: []string{" admin ", "", " user "},
			},
			IpValidation: EndpointIpValidation{
				AllowedIps: []string{" 127.0.0.1 ", "", "10.0.0.1"},
			},
		}

		err := m.Normalize()
		require.NoError(t, err)
		require.Equal(t, "endpoint-id", m.Id)
		require.Equal(t, "POST", m.Method)
		require.Equal(t, "v1/items", m.Path)
		require.Equal(t, "api/items", m.Backend.CustomPath)
		require.Equal(t, []string{"admin", "user"}, m.JwtValidation.Roles)
		require.Equal(t, []string{"127.0.0.1", "10.0.0.1"}, m.IpValidation.AllowedIps)
	})

	t.Run("error empty method", func(t *testing.T) {
		m := Endpoint{
			Method: "   ",
			Path:   "/ok",
			Backend: EndpointBackend{
				CustomPath: "/ok",
			},
		}

		err := m.Normalize()
		require.ErrorIs(t, err, errEmptyValue)
	})
}

func TestEndpointBackendNormalize(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := EndpointBackend{CustomPath: " /backend/path "}
		err := m.Normalize()
		require.NoError(t, err)
		require.Equal(t, "backend/path", m.CustomPath)
	})

	t.Run("error empty path", func(t *testing.T) {
		m := EndpointBackend{CustomPath: " / "}
		err := m.Normalize()
		require.NoError(t, err)
		require.Equal(t, "", m.CustomPath)
	})
}

func TestEndpointJwtValidationNormalize(t *testing.T) {
	m := EndpointJwtValidation{Roles: []string{" role1 ", "", "role2 "}}
	err := m.Normalize()
	require.NoError(t, err)
	require.Equal(t, []string{"role1", "role2"}, m.Roles)
}

func TestEndpointIpValidationNormalize(t *testing.T) {
	m := EndpointIpValidation{AllowedIps: []string{" 1.1.1.1 ", "", "2.2.2.2 "}}
	err := m.Normalize()
	require.NoError(t, err)
	require.Equal(t, []string{"1.1.1.1", "2.2.2.2"}, m.AllowedIps)
}
