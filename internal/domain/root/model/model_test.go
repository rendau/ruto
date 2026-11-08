package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appModel "github.com/rendau/ruto/internal/domain/app/model"
	authModel "github.com/rendau/ruto/internal/domain/auth/model"
	endpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
)

func TestRootJwtNormalize(t *testing.T) {
	tests := []struct {
		name    string
		item    RootJwt
		wantErr string
		check   func(t *testing.T, item RootJwt)
	}{
		{
			name: "plain jwks provider as before",
			item: RootJwt{Name: " corp ", JwkUrl: " http://jwts/jwk/set "},
			check: func(t *testing.T, item RootJwt) {
				assert.Equal(t, "corp", item.Name)
				assert.Equal(t, "http://jwts/jwk/set", item.JwkUrl)
				assert.False(t, item.LoginEnabled())
			},
		},
		{
			name:    "neither jwk_url nor issuer",
			item:    RootJwt{Name: "corp"},
			wantErr: "jwk_url: empty",
		},
		{
			name:    "login settings without an issuer",
			item:    RootJwt{Name: "corp", JwkUrl: "http://jwts/jwk/set", PublicUrl: "https://api.example.com"},
			wantErr: "issuer: empty, but login settings are set",
		},
		{
			name:    "issuer requires a name",
			item:    RootJwt{Issuer: "https://id.example.com/realms/corp"},
			wantErr: "name: empty, required with issuer",
		},
		{
			name:    "issuer must be an absolute http url",
			item:    RootJwt{Name: "corp", Issuer: "id.example.com/realms/corp"},
			wantErr: "issuer: scheme must be http or https",
		},
		{
			name:    "client needs a secret",
			item:    RootJwt{Name: "corp", Issuer: "https://id.example.com/realms/corp", Client: &authModel.JwtClient{Id: "ruto"}},
			wantErr: "client: secret: empty",
		},
		{
			name: "oidc provider without jwk_url, client scopes get openid",
			item: RootJwt{
				Name:        "corp",
				Issuer:      "https://id.example.com/realms/corp/",
				InternalUrl: "http://keycloak:8080/realms/corp/",
				Client:      &authModel.JwtClient{Id: " ruto ", Secret: "s", Scopes: []string{"profile", " ", "email", "profile"}},
			},
			check: func(t *testing.T, item RootJwt) {
				assert.True(t, item.LoginEnabled())
				assert.Equal(t, "https://id.example.com/realms/corp", item.Issuer)
				assert.Equal(t, "http://keycloak:8080/realms/corp", item.InternalUrl)
				assert.Equal(t, "ruto", item.Client.Id)
				assert.Equal(t, []string{"openid", "profile", "email"}, item.Client.Scopes)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.item.Normalize()
			if tt.wantErr != "" {
				require.EqualError(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			tt.check(t, tt.item)
		})
	}
}

func TestRootNormalizeLoginNeedsPublicUrl(t *testing.T) {
	root := NewEmpty()
	root.Jwt = []RootJwt{{Name: "corp", Issuer: "https://id.example.com/realms/corp"}}
	require.EqualError(t, root.Normalize(), "jwt[0]: public_url: empty and root base_url is empty")

	root.BaseUrl = "https://api.example.com/"
	require.NoError(t, root.Normalize())

	root.BaseUrl = ""
	root.Jwt[0].PublicUrl = "https://gw.example.com"
	require.NoError(t, root.Normalize())
}

func TestRootInheritDownJwtCookieNames(t *testing.T) {
	root := NewEmpty()
	root.BaseUrl = "https://api.example.com"
	root.Jwt = []RootJwt{
		{Name: "corp", Issuer: "https://id.example.com/realms/corp", Client: &authModel.JwtClient{Id: "ruto", Secret: "s"}},
		{Name: "partners", JwkUrl: "http://jwts/jwk/set"},
	}
	root.Auth = authModel.Auth{Enabled: true, Methods: []*authModel.AuthMethod{
		{JWT: &authModel.AuthMethodJWT{Provider: "corp", Roles: []string{"shop:admin"}}},
	}}
	shop := &appModel.App{
		Active:     true,
		PathPrefix: "shop/api",
		Backend:    appModel.Backend{Url: "http://shop:8080"},
		Auth:       authModel.Auth{Enabled: true},
		Jwt:        []appModel.AppJwt{{Provider: "corp", Client: authModel.JwtClient{Id: "shop", Secret: "s"}}},
		Endpoints: []*endpointModel.Endpoint{{
			Active: true,
			Http:   endpointModel.Http{Method: "GET", Path: "items"},
			Auth: authModel.Auth{Enabled: true, Methods: []*authModel.AuthMethod{
				{JWT: &authModel.AuthMethodJWT{Provider: "partners"}},
			}},
		}},
	}
	crm := &appModel.App{
		Active:     true,
		PathPrefix: "crm",
		Backend:    appModel.Backend{Url: "http://crm:8080"},
		Auth:       authModel.Auth{Enabled: true},
		Endpoints: []*endpointModel.Endpoint{{
			Active: true,
			Http:   endpointModel.Http{Method: "GET", Path: "deals"},
			Auth:   authModel.Auth{Enabled: true},
		}},
	}
	root.Apps = []*appModel.App{shop, crm}

	require.NoError(t, root.Normalize())
	root.InheritDown()

	// The app with its own client reads its own cookie, scoped to its prefix,
	// for the inherited root method and for its own endpoint method alike.
	shopEp := shop.Endpoints[0]
	require.Len(t, shopEp.Auth.Methods, 2)
	assert.Equal(t, "ruto_corp_shop.api", shopEp.Auth.Methods[0].JWT.CookieName)
	assert.Equal(t, "ruto_partners", shopEp.Auth.Methods[1].JWT.CookieName)
	assert.Equal(t, "ruto_corp_shop.api", shop.Auth.Methods[0].JWT.CookieName)

	// The app on the shared login reads the provider cookie.
	crmEp := crm.Endpoints[0]
	require.Len(t, crmEp.Auth.Methods, 1)
	assert.Equal(t, "ruto_corp", crmEp.Auth.Methods[0].JWT.CookieName)

	// The root's own method is untouched: the cookie name is a per-app matter.
	assert.Empty(t, root.Auth.Methods[0].JWT.CookieName)
}
