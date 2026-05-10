package model

import (
	"fmt"
	"strings"

	"github.com/samber/lo"
)

type Endpoint struct {
	Id            string                `json:"id"`
	Method        string                `json:"method"`
	Path          string                `json:"path"`
	Backend       EndpointBackend       `json:"backend"`
	JwtValidation EndpointJwtValidation `json:"jwt_validation"`
	IpValidation  EndpointIpValidation  `json:"ip_validation"`
}

type EndpointBackend struct {
	CustomPath string `json:"custom_path"`
}

type EndpointJwtValidation struct {
	Enabled bool     `json:"enabled"`
	Roles   []string `json:"roles"`
}

type EndpointIpValidation struct {
	AllowedIps []string `json:"allowed_ips"`
}

func (m *Endpoint) Normalize() error {
	m.Id = strings.TrimSpace(m.Id)

	m.Method = strings.ToUpper(strings.TrimSpace(m.Method))
	if m.Method == "" {
		return fmt.Errorf("method: empty")
	}

	m.Path = strings.Trim(strings.TrimSpace(m.Path), "/")
	if m.Path == "" {
		return fmt.Errorf("path: empty")
	}

	err := m.Backend.Normalize()
	if err != nil {
		return fmt.Errorf("backend: %w", err)
	}

	err = m.JwtValidation.Normalize()
	if err != nil {
		return fmt.Errorf("jwtValidation: %w", err)
	}

	err = m.IpValidation.Normalize()
	if err != nil {
		return fmt.Errorf("ipValidation: %w", err)
	}

	return nil
}

func (m *EndpointBackend) Normalize() error {
	m.CustomPath = strings.TrimPrefix(strings.TrimSpace(m.CustomPath), "/")
	return nil
}

func (m *EndpointJwtValidation) Normalize() error {
	m.Roles = lo.FilterMap(m.Roles, func(v string, _ int) (string, bool) {
		v = strings.TrimSpace(v)
		return v, v != ""
	})
	return nil
}

func (m *EndpointIpValidation) Normalize() error {
	m.AllowedIps = lo.FilterMap(m.AllowedIps, func(v string, _ int) (string, bool) {
		v = strings.TrimSpace(v)
		return v, v != ""
	})
	return nil
}
