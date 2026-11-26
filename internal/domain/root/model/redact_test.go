package model

import (
	"testing"

	"github.com/stretchr/testify/require"

	appModel "github.com/rendau/ruto/internal/domain/app/model"
	authModel "github.com/rendau/ruto/internal/domain/auth/model"
	varsModel "github.com/rendau/ruto/internal/domain/vars/model"
)

func TestRootRedacted(t *testing.T) {
	orig := &Root{
		BaseUrl: "https://api.example.com",
		Jwt:     []RootJwt{{Name: "corp", Issuer: "https://id.example.com"}},
		Variables: varsModel.Vars{
			"BACKEND_TOKEN": "super-secret",
		},
		Auth: authModel.Auth{
			Enabled: true,
			Methods: []*authModel.AuthMethod{
				{
					Basic: &authModel.AuthMethodBasic{
						Users: []authModel.AuthMethodBasicUser{{Username: "admin", Password: "secret-password"}},
					},
				},
				{
					APIKey: &authModel.AuthMethodAPIKey{
						Header: "X-API-Key",
						Keys:   []authModel.AuthMethodAPIKeyItem{{Name: "svc", Key: "secret-key"}},
					},
				},
			},
		},
		Apps: []*appModel.App{
			{
				Id:        "app-1",
				Name:      "shop",
				Variables: varsModel.Vars{"APP_SECRET": "app-secret"},
			},
		},
	}

	r := orig.Redacted()

	require.Equal(t, varsModel.RedactedPlaceholder, r.Variables["BACKEND_TOKEN"])
	require.Equal(t, varsModel.RedactedPlaceholder, r.Auth.Methods[0].Basic.Users[0].Password)
	require.Equal(t, varsModel.RedactedPlaceholder, r.Auth.Methods[1].APIKey.Keys[0].Key)
	require.Equal(t, varsModel.RedactedPlaceholder, r.Apps[0].Variables["APP_SECRET"])

	// Non-secret fields stay visible.
	require.Equal(t, "https://api.example.com", r.BaseUrl)
	require.Equal(t, "admin", r.Auth.Methods[0].Basic.Users[0].Username)
	require.Equal(t, "svc", r.Auth.Methods[1].APIKey.Keys[0].Name)
	require.Equal(t, "corp", r.Jwt[0].Name)

	// Original must not be mutated.
	require.Equal(t, "super-secret", orig.Variables["BACKEND_TOKEN"])
	require.Equal(t, "secret-password", orig.Auth.Methods[0].Basic.Users[0].Password)
	require.Equal(t, "secret-key", orig.Auth.Methods[1].APIKey.Keys[0].Key)
	require.Equal(t, "app-secret", orig.Apps[0].Variables["APP_SECRET"])
}
