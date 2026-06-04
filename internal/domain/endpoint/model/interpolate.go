package model

import (
	varsModel "github.com/rendau/ruto/internal/domain/vars/model"
)

func (m *Endpoint) Interpolate() {
	if len(m.Variables) > 0 {
		m.Backend.Interpolate(m.Variables)
		m.Auth.Interpolate(m.Variables)
	}
}

func (m *Backend) Interpolate(vars varsModel.Vars) {
	m.Headers = vars.InterpolateVars(m.Headers)
	m.QueryParams = vars.InterpolateVars(m.QueryParams)
}
