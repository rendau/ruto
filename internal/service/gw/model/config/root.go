package config

import (
	"fmt"
	"strings"
)

type Root struct {
	PublicBaseUrl string
	Cors          RootCors
	Jwt           []*RootJwt
	Apps          []*App
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
	JwkUrl string
}

func (m *Root) Normalize() error {
	m.PublicBaseUrl = strings.TrimRight(strings.TrimSpace(m.PublicBaseUrl), "/")
	if err := validateNotEmpty(m.PublicBaseUrl); err != nil {
		return fmt.Errorf("public_base_url: %w", err)
	}
	if err := m.Cors.Normalize(); err != nil {
		return fmt.Errorf("cors: %w", err)
	}
	for i := range m.Jwt {
		if err := m.Jwt[i].Normalize(); err != nil {
			return fmt.Errorf("jwt[%d]: %w", i, err)
		}
	}
	for i := range m.Apps {
		if err := m.Apps[i].Normalize(); err != nil {
			return fmt.Errorf("apps[%d]: %w", i, err)
		}
	}
	return nil
}

func (m *RootCors) Normalize() error {
	m.MaxAge = strings.TrimSpace(m.MaxAge)
	m.AllowOrigins = normalizeStringList(m.AllowOrigins)
	m.AllowMethods = normalizeStringList(m.AllowMethods)
	m.AllowHeaders = normalizeStringList(m.AllowHeaders)
	return nil
}

func (m *RootJwt) Normalize() error {
	m.JwkUrl = strings.TrimSpace(m.JwkUrl)
	if err := validateNotEmpty(m.JwkUrl); err != nil {
		return fmt.Errorf("jwk_url: %w", err)
	}
	return nil
}
