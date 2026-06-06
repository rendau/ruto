package model

import (
	"testing"

	"github.com/stretchr/testify/assert"

	varsModel "github.com/rendau/ruto/internal/domain/vars/model"
)

func TestAuth_Interpolate(t *testing.T) {
	vars := varsModel.Vars{
		"user":   "admin",
		"pass":   "secret",
		"key":    "key123",
		"role":   "editor",
		"ip":     "127.0.0.1",
		"header": "X-API-Key",
	}

	auth := &Auth{
		Methods: []*AuthMethod{
			{
				Basic: &AuthMethodBasic{
					Users: []AuthMethodBasicUser{
						{
							Username: "{{user}}",
							Password: "{{pass}}",
						},
					},
				},
			},
			{
				APIKey: &AuthMethodAPIKey{
					Header: "{{header}}",
					Keys:   []AuthMethodAPIKeyItem{{Name: "primary", Key: "{{key}}"}, {Name: "static", Key: "fixed-key"}},
				},
				JWT: &AuthMethodJWT{
					Roles: []string{"{{role}}", "user"},
				},
			},
			{
				JWT: &AuthMethodJWT{
					Roles: []string{"{{role}}", "user"},
				},
			},
			{
				IPValidation: &AuthMethodIPValidation{
					AllowedIps: []AuthMethodIPValidationItem{{Name: "office", Ip: "{{ip}}"}, {Name: "static", Ip: "192.168.1.1"}},
				},
			},
		},
	}

	auth.Interpolate(vars)

	// Check Basic
	assert.Equal(t, "admin", auth.Methods[0].Basic.Users[0].Username)
	assert.Equal(t, "secret", auth.Methods[0].Basic.Users[0].Password)

	// Check APIKey
	assert.Equal(t, "X-API-Key", auth.Methods[1].APIKey.Header)
	assert.Equal(t, []AuthMethodAPIKeyItem{{Name: "primary", Key: "key123"}, {Name: "static", Key: "fixed-key"}}, auth.Methods[1].APIKey.Keys)
	assert.Equal(t, []string{"editor", "user"}, auth.Methods[1].JWT.Roles)

	// Check JWT
	assert.Equal(t, []string{"editor", "user"}, auth.Methods[2].JWT.Roles)

	// Check IPValidation
	assert.Equal(t, []AuthMethodIPValidationItem{{Name: "office", Ip: "127.0.0.1"}, {Name: "static", Ip: "192.168.1.1"}}, auth.Methods[3].IPValidation.AllowedIps)
}

func TestAuthMethod_Interpolate_NilChecks(t *testing.T) {
	vars := varsModel.Vars{"v": "1"}

	// Ensure no panics when fields are nil
	method := &AuthMethod{}
	assert.NotPanics(t, func() {
		method.Interpolate(vars)
	})
}
