package model

import (
	"testing"

	authModel "github.com/rendau/ruto/internal/domain/auth/model"
)

func TestNormalizeList_RejectDuplicateKeysWithinSameLevel(t *testing.T) {
	_, err := NormalizeList([]Variable{
		{Key: " token ", Value: "root"},
		{Key: "token", Value: "app"},
	})
	if err == nil {
		t.Fatalf("NormalizeList() expected duplicate error, got nil")
	}
	if err.Error() != `[1].key: duplicate "token"` {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolve_UsesMergedScopeForVariableValues(t *testing.T) {
	variables := Merge(
		[]Variable{
			{Key: "token", Value: "root-token"},
			{Key: "composed", Value: "{{token}}:{{tenant}}"},
		},
		[]Variable{
			{Key: "tenant", Value: "app-tenant"},
			{Key: "token", Value: "app-token"},
		},
	)
	variables = Merge(variables, []Variable{
		{Key: "token", Value: "endpoint-token"},
	})

	scope, err := Resolve(variables)
	if err != nil {
		t.Fatalf("Resolve() unexpected error: %v", err)
	}
	if scope["token"] != "endpoint-token" {
		t.Fatalf("child override failed: %#v", scope)
	}
	if scope["composed"] != "endpoint-token:app-tenant" {
		t.Fatalf("merged scope resolution failed: %#v", scope)
	}
}

func TestResolve_RejectCycles(t *testing.T) {
	_, err := Resolve([]Variable{
		{Key: "a", Value: "${b}"},
		{Key: "b", Value: "${a}"},
	})
	if err == nil {
		t.Fatalf("Resolve() expected cycle error, got nil")
	}
}

func TestInterpolateString_RejectUnknownVariable(t *testing.T) {
	_, err := InterpolateString("{{missing}}", map[string]string{})
	if err == nil {
		t.Fatalf("InterpolateString() expected unknown variable error, got nil")
	}
}

func TestInterpolateAuth_InterpolatesSupportedAuthConfigs(t *testing.T) {
	auth, err := InterpolateAuth(authModel.Auth{
		Enabled: true,
		Mode:    "extend",
		Methods: []*authModel.AuthMethod{
			{
				APIKey: &authModel.AuthMethodAPIKey{
					Header: "{{api_header}}",
					Keys:   []string{"{{api_key}}"},
				},
			},
			{
				Basic: &authModel.AuthMethodBasic{
					Users: []authModel.AuthMethodBasicUser{
						{Username: "{{basic_user}}", Password: "{{basic_pass}}"},
					},
				},
			},
			{
				IPValidation: &authModel.AuthMethodIPValidation{
					AllowedIps: []string{"{{allowed_ip}}"},
				},
			},
		},
	}, map[string]string{
		"api_header": "X-Api-Key",
		"api_key":    "secret",
		"basic_user": "admin",
		"basic_pass": "password",
		"allowed_ip": "10.0.0.1",
	})
	if err != nil {
		t.Fatalf("InterpolateAuth() unexpected error: %v", err)
	}

	if auth.Methods[0].APIKey.Header != "X-Api-Key" || auth.Methods[0].APIKey.Keys[0] != "secret" {
		t.Fatalf("api key interpolation failed: %#v", auth.Methods[0].APIKey)
	}
	if auth.Methods[1].Basic.Users[0].Username != "admin" || auth.Methods[1].Basic.Users[0].Password != "password" {
		t.Fatalf("basic interpolation failed: %#v", auth.Methods[1].Basic.Users[0])
	}
	if auth.Methods[2].IPValidation.AllowedIps[0] != "10.0.0.1" {
		t.Fatalf("ip validation interpolation failed: %#v", auth.Methods[2].IPValidation)
	}
}
