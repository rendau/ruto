package model

import (
	varsModel "github.com/rendau/ruto/internal/domain/vars/model"
)

// Redacted returns a copy of the auth config with all secret material (basic
// passwords, api keys) masked. Non-secret data (usernames, key names, headers,
// kid, roles, allowed ips) is preserved. The receiver is left untouched.
func (m *Auth) Redacted() Auth {
	result := Auth{
		Enabled: m.Enabled,
		Mode:    m.Mode,
		Methods: m.CloneMethods(),
	}

	for _, method := range result.Methods {
		if method.Basic != nil {
			for i := range method.Basic.Users {
				method.Basic.Users[i].Password = varsModel.RedactedPlaceholder
			}
		}
		if method.APIKey != nil {
			for i := range method.APIKey.Keys {
				method.APIKey.Keys[i].Key = varsModel.RedactedPlaceholder
			}
		}
	}

	return result
}
