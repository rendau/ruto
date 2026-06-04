package model

import (
	varsModel "github.com/rendau/ruto/internal/domain/vars/model"
)

func (m *App) Interpolate() {
	if len(m.Variables) > 0 {
		m.Backend.Interpolate(m.Variables)
		m.Auth.Interpolate(m.Variables)
	}

	for _, ep := range m.Endpoints {
		ep.Interpolate()
	}
}

func (m *Backend) Interpolate(vars varsModel.Vars) {
	m.Headers = vars.InterpolateVars(m.Headers)
	m.QueryParams = vars.InterpolateVars(m.QueryParams)
}
