package model

import (
	"testing"

	"github.com/stretchr/testify/require"

	authModel "github.com/rendau/ruto/internal/domain/auth/model"
	varsModel "github.com/rendau/ruto/internal/domain/vars/model"
)

func TestEndpointRedacted(t *testing.T) {
	ep := &Endpoint{
		Id:    "ep-1",
		AppId: "app-1",
		Http:  Http{Method: "GET", Path: "/users"},
		Backend: Backend{
			Headers:     varsModel.Vars{"Authorization": "Bearer real"},
			QueryParams: varsModel.Vars{"token": "real-token"},
		},
		Auth: authModel.Auth{
			Methods: []*authModel.AuthMethod{
				{Basic: &authModel.AuthMethodBasic{Users: []authModel.AuthMethodBasicUser{{Username: "u", Password: "real-pass"}}}},
			},
		},
		Variables: varsModel.Vars{"secret": "secret-val"},
	}

	r := ep.Redacted()

	// non-secret data preserved
	require.Equal(t, "ep-1", r.Id)
	require.Equal(t, "app-1", r.AppId)
	require.Equal(t, "/users", r.Http.Path)
	require.Equal(t, "u", r.Auth.Methods[0].Basic.Users[0].Username)

	// secrets masked
	require.Equal(t, varsModel.RedactedPlaceholder, r.Variables["secret"])
	require.Equal(t, varsModel.RedactedPlaceholder, r.Backend.Headers["Authorization"])
	require.Equal(t, varsModel.RedactedPlaceholder, r.Backend.QueryParams["token"])
	require.Equal(t, varsModel.RedactedPlaceholder, r.Auth.Methods[0].Basic.Users[0].Password)

	// the source is left untouched
	require.Equal(t, "secret-val", ep.Variables["secret"])
	require.Equal(t, "real-pass", ep.Auth.Methods[0].Basic.Users[0].Password)
}
