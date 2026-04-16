package config

import (
	"fmt"
	"strings"
)

type Endpoint struct {
	Id            string
	Method        string
	Path          string
	Backend       EndpointBackend
	JwtValidation EndpointJwtValidation
	IpValidation  EndpointIpValidation
}

type EndpointBackend struct {
	Path string
}

type EndpointJwtValidation struct {
	Enabled bool
	Roles   []string
}

type EndpointIpValidation struct {
	AllowedIps []string
}

func (m *Endpoint) Normalize() {
	m.Id = strings.TrimSpace(m.Id)
	m.Method = strings.ToUpper(strings.TrimSpace(m.Method))
	m.Path = strings.TrimPrefix(strings.TrimSpace(m.Path), "/")

	m.Backend.Normalize()
	m.JwtValidation.Normalize()
	m.IpValidation.Normalize()
}

func (m *EndpointBackend) Normalize() {
	m.Path = strings.TrimPrefix(strings.TrimSpace(m.Path), "/")
}

func (m *EndpointJwtValidation) Normalize() {
	m.Roles = normalizeStringList(m.Roles)
}

func (m *EndpointIpValidation) Normalize() {
	m.AllowedIps = normalizeStringList(m.AllowedIps)
}

func (m *Endpoint) Validate() error {
	if err := validateNotEmpty(m.Id); err != nil {
		return fmt.Errorf("id: %w", err)
	}
	if err := validateNotEmpty(m.Method); err != nil {
		return fmt.Errorf("method: %w", err)
	}
	if err := validateNotEmpty(m.Path); err != nil {
		return fmt.Errorf("path: %w", err)
	}
	if err := m.Backend.Validate(); err != nil {
		return fmt.Errorf("backend: %w", err)
	}
	if err := m.JwtValidation.Validate(); err != nil {
		return fmt.Errorf("jwt_validation: %w", err)
	}
	if err := m.IpValidation.Validate(); err != nil {
		return fmt.Errorf("ip_validation: %w", err)
	}
	return nil
}

func (m *EndpointBackend) Validate() error {
	if err := validateNotEmpty(m.Path); err != nil {
		return fmt.Errorf("path: %w", err)
	}
	return nil
}

func (m *EndpointJwtValidation) Validate() error {
	return nil
}

func (m *EndpointIpValidation) Validate() error {
	return nil
}
