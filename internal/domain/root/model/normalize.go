package model

import (
	"fmt"
	"strings"

	"github.com/samber/lo"
)

func (m *Root) Normalize() error {
	m.BaseUrl = strings.TrimRight(strings.TrimSpace(m.BaseUrl), "/")
	if err := m.Cors.Normalize(); err != nil {
		return fmt.Errorf("cors: %w", err)
	}
	for i := range m.Jwt {
		if err := m.Jwt[i].Normalize(); err != nil {
			return fmt.Errorf("jwt[%d]: %w", i, err)
		}
	}
	if err := m.Auth.Normalize(); err != nil {
		return fmt.Errorf("auth: %w", err)
	}
	if err := m.Variables.Normalize(); err != nil {
		return fmt.Errorf("variables: %w", err)
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
	m.AllowOrigins = lo.FilterMap(m.AllowOrigins, func(v string, _ int) (string, bool) {
		v = strings.TrimSpace(v)
		return v, v != ""
	})
	m.AllowMethods = lo.FilterMap(m.AllowMethods, func(v string, _ int) (string, bool) {
		v = strings.ToUpper(strings.TrimSpace(v))
		return v, v != ""
	})
	m.AllowHeaders = lo.FilterMap(m.AllowHeaders, func(v string, _ int) (string, bool) {
		v = strings.TrimSpace(v)
		return v, v != ""
	})
	return nil
}

func (m *RootJwt) Normalize() error {
	m.JwkUrl = strings.TrimSpace(m.JwkUrl)
	if m.JwkUrl == "" {
		return fmt.Errorf("jwk_url: empty")
	}
	return nil
}
