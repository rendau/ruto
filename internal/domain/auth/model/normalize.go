package model

import (
	"fmt"
	"strings"

	"github.com/samber/lo"

	"github.com/rendau/ruto/internal/constant"
)

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
	if m.Header == "" {
		m.Header = "X-Api-Key"
	}
	m.Keys = lo.FilterMap(m.Keys, func(v AuthMethodAPIKeyItem, _ int) (AuthMethodAPIKeyItem, bool) {
		v.Name = strings.TrimSpace(v.Name)
		v.Key = strings.TrimSpace(v.Key)
		return v, v.Key != ""
	})
	if len(m.Keys) == 0 {
		return fmt.Errorf("keys: empty")
	}
	return nil
}

func (m *AuthMethodJWT) Normalize() error {
	m.Kid = strings.TrimSpace(m.Kid)
	if m.Kid == "" {
		return fmt.Errorf("kid: empty")
	}
	m.Roles = lo.FilterMap(m.Roles, func(v string, _ int) (string, bool) {
		v = strings.TrimSpace(v)
		return v, v != ""
	})
	m.Roles = lo.Uniq(m.Roles)
	return nil
}

func (m *AuthMethodIPValidation) Normalize() error {
	m.AllowedIps = lo.FilterMap(m.AllowedIps, func(v AuthMethodIPValidationItem, _ int) (AuthMethodIPValidationItem, bool) {
		v.Name = strings.TrimSpace(v.Name)
		v.Ip = strings.TrimSpace(v.Ip)
		return v, v.Ip != ""
	})
	if len(m.AllowedIps) == 0 {
		return fmt.Errorf("allowed_ips: empty")
	}
	return nil
}
