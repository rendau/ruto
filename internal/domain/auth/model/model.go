package model

import (
	"fmt"
	"strings"

	"github.com/samber/lo"

	"github.com/rendau/ruto/internal/constant"
)

type Auth struct {
	Enabled bool          `json:"enabled"`
	Mode    string        `json:"mode"` // "extend" | "replace"
	Methods []*AuthMethod `json:"methods"`
}

type AuthMethod struct {
	Basic        *AuthMethodBasic        `json:"basic,omitempty"`
	APIKey       *AuthMethodAPIKey       `json:"api_key,omitempty"`
	JWT          *AuthMethodJWT          `json:"jwt,omitempty"`
	IPValidation *AuthMethodIPValidation `json:"ip_validation,omitempty"`
}

type AuthMethodBasic struct {
	Users []AuthMethodBasicUser `json:"users"`
}

type AuthMethodBasicUser struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type AuthMethodAPIKey struct {
	Header string   `json:"header"`
	Keys   []string `json:"keys"`
}

type AuthMethodJWT struct {
	Kids  []string `json:"kids"`
	Roles []string `json:"roles"`
}

type AuthMethodIPValidation struct {
	AllowedIps []string `json:"allowed_ips"`
}

func (m *Auth) Normalize() error {
	m.Mode = strings.ToLower(strings.TrimSpace(m.Mode))
	if m.Mode == "" {
		m.Mode = constant.AuthModeExtend
	}
	if !constant.AuthModeIsValid(m.Mode) {
		return fmt.Errorf("mode: is invalid")
	}

	if m.Methods == nil {
		m.Methods = []*AuthMethod{}
	}
	for i := range m.Methods {
		if err := m.Methods[i].Normalize(); err != nil {
			return fmt.Errorf("methods[%d]: %w", i, err)
		}
	}

	return nil
}

func (m *Auth) Merge(rootAuth, appAuth *Auth) {
	result := Auth{}

	result.mergeOne(rootAuth)
	result.mergeOne(appAuth)
	result.mergeOne(m)

	*m = result
}

func (m *Auth) mergeOne(child *Auth) {
	if child == nil {
		return
	}

	switch child.Mode {
	case constant.AuthModeReplace:
		*m = Auth{
			Enabled: child.Enabled,
			Mode:    child.Mode,
			Methods: append(make([]*AuthMethod, 0, len(child.Methods)), child.Methods...),
		}
	case constant.AuthModeExtend:
		m.Enabled = child.Enabled
		m.Mode = child.Mode
		m.Methods = append(m.Methods, child.Methods...)
	}
}

func (m *AuthMethod) Normalize() error {
	var methodsCount int

	if m.Basic != nil {
		methodsCount++
		if err := m.Basic.Normalize(); err != nil {
			return fmt.Errorf("basic: %w", err)
		}
	}
	if m.APIKey != nil {
		methodsCount++
		if err := m.APIKey.Normalize(); err != nil {
			return fmt.Errorf("api_key: %w", err)
		}
	}
	if m.JWT != nil {
		methodsCount++
		if err := m.JWT.Normalize(); err != nil {
			return fmt.Errorf("jwt: %w", err)
		}
	}
	if m.IPValidation != nil {
		methodsCount++
		if err := m.IPValidation.Normalize(); err != nil {
			return fmt.Errorf("ip_validation: %w", err)
		}
	}

	if methodsCount == 0 {
		return fmt.Errorf("empty")
	}

	return nil
}

func (m *AuthMethodBasic) Normalize() error {
	for i := range m.Users {
		if err := m.Users[i].Normalize(); err != nil {
			return fmt.Errorf("users[%d]: %w", i, err)
		}
	}
	return nil
}

func (m *AuthMethodBasicUser) Normalize() error {
	m.Username = strings.TrimSpace(m.Username)
	if m.Username == "" {
		return fmt.Errorf("username: empty")
	}

	m.Password = strings.TrimSpace(m.Password)

	return nil
}

func (m *AuthMethodAPIKey) Normalize() error {
	m.Header = strings.TrimSpace(m.Header)
	m.Keys = lo.FilterMap(m.Keys, func(v string, _ int) (string, bool) {
		v = strings.TrimSpace(v)
		return v, v != ""
	})
	return nil
}

func (m *AuthMethodJWT) Normalize() error {
	m.Kids = lo.FilterMap(m.Kids, func(v string, _ int) (string, bool) {
		v = strings.TrimSpace(v)
		return v, v != ""
	})
	m.Roles = lo.FilterMap(m.Roles, func(v string, _ int) (string, bool) {
		v = strings.TrimSpace(v)
		return v, v != ""
	})
	return nil
}

func (m *AuthMethodIPValidation) Normalize() error {
	m.AllowedIps = lo.FilterMap(m.AllowedIps, func(v string, _ int) (string, bool) {
		v = strings.TrimSpace(v)
		return v, v != ""
	})
	return nil
}
