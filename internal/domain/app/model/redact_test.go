package model

import (
	"testing"

	"github.com/stretchr/testify/require"

	authModel "github.com/rendau/ruto/internal/domain/auth/model"
	endpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
	varsModel "github.com/rendau/ruto/internal/domain/vars/model"
)

func TestAppRedacted(t *testing.T) {
	app := &App{
		Id:         "app-1",
		Name:       "App One",
		PathPrefix: "/one",
		Backend: Backend{
			Url:         "https://backend.local",
			Headers:     varsModel.Vars{"Authorization": "Bearer real"},
			QueryParams: varsModel.Vars{"token": "real-token"},
		},
		Auth: authModel.Auth{
			Methods: []*authModel.AuthMethod{
				{APIKey: &authModel.AuthMethodAPIKey{Header: "X-Key", Keys: []authModel.AuthMethodAPIKeyItem{{Name: "n", Key: "real-key"}}}},
			},
		},
		Variables: varsModel.Vars{"secret": "secret-val"},
		Endpoints: []*endpointModel.Endpoint{
			{Id: "ep-1", AppId: "app-1", Variables: varsModel.Vars{"ep_secret": "ep-val"}},
		},
	}

	r := app.Redacted()

	// non-secret data preserved
	require.Equal(t, "app-1", r.Id)
	require.Equal(t, "App One", r.Name)
	require.Equal(t, "https://backend.local", r.Backend.Url)

	// secrets masked (keys are kept, only values are masked)
	require.Equal(t, varsModel.RedactedPlaceholder, r.Variables["secret"])
	require.Contains(t, r.Backend.Headers, "Authorization")
	require.Equal(t, varsModel.RedactedPlaceholder, r.Backend.Headers["Authorization"])
	require.Equal(t, varsModel.RedactedPlaceholder, r.Backend.QueryParams["token"])
	require.Equal(t, varsModel.RedactedPlaceholder, r.Auth.Methods[0].APIKey.Keys[0].Key)
	require.Equal(t, varsModel.RedactedPlaceholder, r.Endpoints[0].Variables["ep_secret"])

	// the source is left untouched
	require.Equal(t, "secret-val", app.Variables["secret"])
	require.Equal(t, "Bearer real", app.Backend.Headers["Authorization"])
	require.Equal(t, "real-key", app.Auth.Methods[0].APIKey.Keys[0].Key)
	require.Equal(t, "ep-val", app.Endpoints[0].Variables["ep_secret"])
}
