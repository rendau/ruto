package model

import (
	"fmt"
	"strings"

	"github.com/samber/lo"

	commonModel "github.com/rendau/ruto/internal/domain/common/model"
)

type Endpoint struct {
	Id           string       `json:"id"`
	AppId        string       `json:"app_id"`
	Active       bool         `json:"active"`
	Method       string       `json:"method"`
	Path         string       `json:"path"`
	Backend      Backend      `json:"backend"`
	Auth         Auth         `json:"auth"`
	IpValidation IpValidation `json:"ip_validation"`
}

// backend

type (
	Backend struct {
		CustomPath string `json:"custom_path"`
	}
)

// auth

type (
	Auth struct {
		Enabled bool         `json:"enabled"`
		Methods []AuthMethod `json:"methods"`
	}

	AuthMethod struct {
		Basic  *AuthMethodBasic  `json:"basic,omitempty"`
		APIKey *AuthMethodAPIKey `json:"api_key,omitempty"`
		JWT    *AuthMethodJWT    `json:"jwt,omitempty"`
	}

	AuthMethodBasic struct {
		Users []AuthMethodBasicUser `json:"users"`
	}

	AuthMethodBasicUser struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	AuthMethodAPIKey struct {
		Header string   `json:"header"`
		Keys   []string `json:"keys"`
	}

	AuthMethodJWT struct {
		Kids  []string `json:"kids"`
		Roles []string `json:"roles"`
	}
)

// ip validation

type (
	IpValidation struct {
		AllowedIps []string `json:"allowed_ips"`
	}
)

// string

func (m *Endpoint) String() string {
	return fmt.Sprintf("endpoint{%s %s}", m.Method, m.Path)
}

// normalize

func (m *Endpoint) Normalize() error {
	m.Method = strings.ToUpper(strings.TrimSpace(m.Method))
	if m.Method == "" {
		return fmt.Errorf("method: empty")
	}

	m.Path = strings.Trim(strings.TrimSpace(m.Path), "/")
	if m.Path == "" {
		return fmt.Errorf("path: empty")
	}

	if err := m.Backend.Normalize(); err != nil {
		return fmt.Errorf("backend: %w", err)
	}
	if err := m.Auth.Normalize(); err != nil {
		return fmt.Errorf("auth: %w", err)
	}
	if err := m.IpValidation.Normalize(); err != nil {
		return fmt.Errorf("ip_validation: %w", err)
	}

	return nil
}

func (m *Backend) Normalize() error {
	m.CustomPath = strings.TrimPrefix(strings.TrimSpace(m.CustomPath), "/")
	return nil
}

func (m *Auth) Normalize() error {
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

	if methodsCount == 0 {
		return fmt.Errorf("empty")
	}
	if methodsCount > 1 {
		return fmt.Errorf("must contain exactly one auth method")
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

func (m *IpValidation) Normalize() error {
	m.AllowedIps = lo.FilterMap(m.AllowedIps, func(v string, _ int) (string, bool) {
		v = strings.TrimSpace(v)
		return v, v != ""
	})
	return nil
}

type ListReq struct {
	commonModel.ListParams

	AppId  *string
	Active *bool
}
