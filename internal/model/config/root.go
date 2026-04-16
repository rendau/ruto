package config

import (
	"fmt"
	"strings"
	"time"
)

type Root struct {
	PublicBaseUrl string
	Timeout       RootTimeout
	Cors          RootCors
	Jwt           []RootJwt
	Apps          []App
}

type RootTimeout struct {
	Global     time.Duration
	ReadHeader time.Duration
	Read       time.Duration
}

type RootCors struct {
	Enabled          bool
	AllowCredentials bool
	MaxAge           string
	AllowOrigins     []string
	AllowMethods     []string
	AllowHeaders     []string
}

type RootJwt struct {
	JwkUrl        string
	Alg           string
	CacheDuration time.Duration
	RolesPath     string
}

func (m *Root) Normalize() {
	m.PublicBaseUrl = strings.TrimRight(strings.TrimSpace(m.PublicBaseUrl), "/")

	m.Timeout.Normalize()
	m.Cors.Normalize()
	for i := range m.Jwt {
		m.Jwt[i].Normalize()
	}
	for i := range m.Apps {
		m.Apps[i].Normalize()
	}
}

func (m *RootTimeout) Normalize() {
}

func (m *RootCors) Normalize() {
	m.MaxAge = strings.TrimSpace(m.MaxAge)
	m.AllowOrigins = normalizeStringList(m.AllowOrigins)
	m.AllowMethods = normalizeStringList(m.AllowMethods)
	m.AllowHeaders = normalizeStringList(m.AllowHeaders)
}

func (m *RootJwt) Normalize() {
	m.JwkUrl = strings.TrimSpace(m.JwkUrl)
	m.Alg = strings.ToUpper(strings.TrimSpace(m.Alg))
	m.RolesPath = strings.TrimSpace(m.RolesPath)
}

func (m *Root) Validate() error {
	if err := validateNotEmpty(m.PublicBaseUrl); err != nil {
		return fmt.Errorf("public_base_url: %w", err)
	}
	if err := m.Timeout.Validate(); err != nil {
		return fmt.Errorf("timeout: %w", err)
	}
	if err := m.Cors.Validate(); err != nil {
		return fmt.Errorf("cors: %w", err)
	}
	for i := range m.Jwt {
		if err := m.Jwt[i].Validate(); err != nil {
			return fmt.Errorf("jwt[%d]: %w", i, err)
		}
	}
	for i := range m.Apps {
		if err := m.Apps[i].Validate(); err != nil {
			return fmt.Errorf("apps[%d]: %w", i, err)
		}
	}
	return nil
}

func (m *RootTimeout) Validate() error {
	if err := validateNonNegative(m.Global); err != nil {
		return fmt.Errorf("global: %w", err)
	}
	if err := validateNonNegative(m.ReadHeader); err != nil {
		return fmt.Errorf("read_header: %w", err)
	}
	if err := validateNonNegative(m.Read); err != nil {
		return fmt.Errorf("read: %w", err)
	}
	return nil
}

func (m *RootCors) Validate() error {
	return nil
}

func (m *RootJwt) Validate() error {
	if err := validateNotEmpty(m.JwkUrl); err != nil {
		return fmt.Errorf("jwk_url: %w", err)
	}
	if err := validateNotEmpty(m.Alg); err != nil {
		return fmt.Errorf("alg: %w", err)
	}
	if err := validateNonNegative(m.CacheDuration); err != nil {
		return fmt.Errorf("cache_duration: %w", err)
	}
	if err := validateNotEmpty(m.RolesPath); err != nil {
		return fmt.Errorf("roles_path: %w", err)
	}
	return nil
}
