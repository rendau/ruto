package model

import (
	"testing"

	"github.com/stretchr/testify/require"

	varsModel "github.com/rendau/ruto/internal/domain/vars/model"
)

func TestAuthRedacted(t *testing.T) {
	a := Auth{
		Enabled: true,
		Mode:    "extend",
		Methods: []*AuthMethod{
			{Basic: &AuthMethodBasic{Users: []AuthMethodBasicUser{{Username: "u", Password: "secret-pass"}}}},
			{APIKey: &AuthMethodAPIKey{Header: "X-Key", Keys: []AuthMethodAPIKeyItem{{Name: "n", Key: "secret-key"}}}},
			{JWT: &AuthMethodJWT{Kid: "kid-1", Roles: []string{"admin"}}},
		},
	}

	r := a.Redacted()

	require.True(t, r.Enabled)
	require.Equal(t, "extend", r.Mode)
	// secrets masked
	require.Equal(t, varsModel.RedactedPlaceholder, r.Methods[0].Basic.Users[0].Password)
	require.Equal(t, varsModel.RedactedPlaceholder, r.Methods[1].APIKey.Keys[0].Key)
	// non-secret data preserved
	require.Equal(t, "u", r.Methods[0].Basic.Users[0].Username)
	require.Equal(t, "X-Key", r.Methods[1].APIKey.Header)
	require.Equal(t, "n", r.Methods[1].APIKey.Keys[0].Name)
	require.Equal(t, "kid-1", r.Methods[2].JWT.Kid)
	require.Equal(t, []string{"admin"}, r.Methods[2].JWT.Roles)

	// the source is left untouched
	require.Equal(t, "secret-pass", a.Methods[0].Basic.Users[0].Password)
	require.Equal(t, "secret-key", a.Methods[1].APIKey.Keys[0].Key)
}
