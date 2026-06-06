package model

import (
	"github.com/samber/lo"

	varsModel "github.com/rendau/ruto/internal/domain/vars/model"
)

func (m *Auth) Interpolate(vars varsModel.Vars) {
	lo.ForEach(m.Methods, func(method *AuthMethod, _ int) {
		method.Interpolate(vars)
	})
}

func (m *AuthMethod) Interpolate(vars varsModel.Vars) {
	if m.Basic != nil {
		m.Basic.Interpolate(vars)
	}
	if m.APIKey != nil {
		m.APIKey.Interpolate(vars)
	}
	if m.JWT != nil {
		m.JWT.Interpolate(vars)
	}
	if m.IPValidation != nil {
		m.IPValidation.Interpolate(vars)
	}
}

func (m *AuthMethodBasic) Interpolate(vars varsModel.Vars) {
	for i := range m.Users {
		m.Users[i].Interpolate(vars)
	}
}

func (m *AuthMethodBasicUser) Interpolate(vars varsModel.Vars) {
	m.Username = vars.InterpolateString(m.Username)
	m.Password = vars.InterpolateString(m.Password)
}

func (m *AuthMethodAPIKey) Interpolate(vars varsModel.Vars) {
	m.Header = vars.InterpolateString(m.Header)
	for i := range m.Keys {
		m.Keys[i].Key = vars.InterpolateString(m.Keys[i].Key)
	}
}

func (m *AuthMethodJWT) Interpolate(vars varsModel.Vars) {
	m.Roles = vars.InterpolateStrings(m.Roles)
}

func (m *AuthMethodIPValidation) Interpolate(vars varsModel.Vars) {
	for i := range m.AllowedIps {
		m.AllowedIps[i].Ip = vars.InterpolateString(m.AllowedIps[i].Ip)
	}
}
