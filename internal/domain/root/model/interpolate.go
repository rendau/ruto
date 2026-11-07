package model

import (
	varsModel "github.com/rendau/ruto/internal/domain/vars/model"
)

func (m *Root) Interpolate() {
	if len(m.Variables) > 0 {
		m.Auth.Interpolate(m.Variables)
		for i := range m.Jwt {
			m.Jwt[i].Interpolate(m.Variables)
		}
	}

	for _, app := range m.Apps {
		app.Interpolate()
	}
}

// Interpolate resolves variables in the client secret, so it can live in Root
// variables like any other secret.
func (m *RootJwt) Interpolate(vars varsModel.Vars) {
	if m.Client != nil {
		m.Client.Interpolate(vars)
	}
}
