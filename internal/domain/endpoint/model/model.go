package model

import (
	"fmt"
	"strings"

	"github.com/samber/lo"

	commonModel "github.com/rendau/ruto/internal/domain/common/model"
)

type Endpoint struct {
	Id            string        `json:"id"`
	AppId         string        `json:"app_id"`
	Active        bool          `json:"active"`
	Method        string        `json:"method"`
	Path          string        `json:"path"`
	Backend       Backend       `json:"backend"`
	JwtValidation JwtValidation `json:"jwt_validation"`
	IpValidation  IpValidation  `json:"ip_validation"`
}

type Backend struct {
	CustomPath string `json:"custom_path"`
}

type JwtValidation struct {
	Enabled bool     `json:"enabled"`
	Roles   []string `json:"roles"`
}

type IpValidation struct {
	AllowedIps []string `json:"allowed_ips"`
}

func (m *Endpoint) String() string {
	return fmt.Sprintf("endpoint{%s %s}", m.Method, m.Path)
}

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
	if err := m.JwtValidation.Normalize(); err != nil {
		return fmt.Errorf("jwt_validation: %w", err)
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

func (m *JwtValidation) Normalize() error {
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
