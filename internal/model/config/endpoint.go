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

func (m *Endpoint) Normalize() error {
	m.Id = strings.TrimSpace(m.Id)

	m.Method = strings.ToUpper(strings.TrimSpace(m.Method))
	if err := validateNotEmpty(m.Method); err != nil {
		return fmt.Errorf("method: %w", err)
	}

	m.Path = strings.TrimPrefix(strings.TrimSpace(m.Path), "/")
	if err := validateNotEmpty(m.Path); err != nil {
		return fmt.Errorf("path: %w", err)
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
	m.Path = strings.TrimPrefix(strings.TrimSpace(m.Path), "/")
	return nil
}

func (m *EndpointJwtValidation) Normalize() error {
	m.Roles = normalizeStringList(m.Roles)
	return nil
}

func (m *EndpointIpValidation) Normalize() error {
	m.AllowedIps = normalizeStringList(m.AllowedIps)
	return nil
}
