package model

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRootJwtNormalize(t *testing.T) {
	tests := []struct {
		name    string
		item    RootJwt
		wantErr string
		want    RootJwt
	}{
		{
			name: "plain jwks provider",
			item: RootJwt{Name: " corp ", JwkUrl: " http://jwts/jwk/set "},
			want: RootJwt{Name: "corp", JwkUrl: "http://jwts/jwk/set"},
		},
		{
			name: "keys from discovery by issuer, read from the internal address",
			item: RootJwt{Name: "corp", Issuer: "https://id.example.com/realms/corp/", InternalUrl: "http://keycloak:8080/realms/corp/"},
			want: RootJwt{Name: "corp", Issuer: "https://id.example.com/realms/corp", InternalUrl: "http://keycloak:8080/realms/corp"},
		},
		{
			name:    "neither jwk_url nor issuer",
			item:    RootJwt{Name: "corp"},
			wantErr: "jwk_url: empty",
		},
		{
			name:    "internal_url without issuer",
			item:    RootJwt{JwkUrl: "http://jwts/jwk/set", InternalUrl: "http://keycloak:8080"},
			wantErr: "internal_url: set without issuer",
		},
		{
			name:    "issuer must be an absolute http url",
			item:    RootJwt{Name: "corp", Issuer: "id.example.com/realms/corp"},
			wantErr: "issuer: scheme must be http or https",
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
			assert.Equal(t, tt.want, tt.item)
		})
	}
}

// TestRootReadsOldLoginFields: roots saved while the gateway logged users in
// carry provider and app fields that are gone now; they are read without an
// error and dropped on the next save.
func TestRootReadsOldLoginFields(t *testing.T) {
	const stored = `{
		"base_url": "https://api.example.com",
		"jwt": [{
			"name": "id",
			"issuer": "https://id.example.com/realms/corp",
			"public_url": "https://api.example.com",
			"client": {"id": "ruto", "secret": "s", "scopes": ["openid"]},
			"exchange_scopes": ["roles"],
			"clients": [{"id": "crm"}]
		}]
	}`
	root := NewEmpty()
	require.NoError(t, json.Unmarshal([]byte(stored), root))
	require.NoError(t, root.Normalize())
	assert.Equal(t, []RootJwt{{Name: "id", Issuer: "https://id.example.com/realms/corp"}}, root.Jwt)

	saved, err := json.Marshal(root.Jwt)
	require.NoError(t, err)
	assert.JSONEq(t, `[{"name":"id","jwk_url":"","issuer":"https://id.example.com/realms/corp","internal_url":""}]`, string(saved))
}
