package model

import (
	"fmt"
	"strings"

	"github.com/samber/lo"

	compressionModel "github.com/rendau/ruto/internal/domain/compression/model"
)

func (m *Root) Normalize() error {
	m.BaseUrl = strings.TrimRight(strings.TrimSpace(m.BaseUrl), "/")
	if err := m.Cors.Normalize(); err != nil {
		return fmt.Errorf("cors: %w", err)
	}
	providerNames := make(map[string]struct{}, len(m.Jwt))
	for i := range m.Jwt {
		if err := m.Jwt[i].Normalize(); err != nil {
			return fmt.Errorf("jwt[%d]: %w", i, err)
		}
		if name := m.Jwt[i].Name; name != "" {
			if _, ok := providerNames[name]; ok {
				return fmt.Errorf("jwt[%d]: name: duplicate %q", i, name)
			}
			providerNames[name] = struct{}{}
		}
	}
	if err := m.Auth.Normalize(); err != nil {
		return fmt.Errorf("auth: %w", err)
	}
	if err := m.Logging.Normalize(); err != nil {
		return fmt.Errorf("logging: %w", err)
	}
	if err := m.Variables.Normalize(); err != nil {
		return fmt.Errorf("variables: %w", err)
	}
	if m.Transform.MaxWorkers < 0 {
		m.Transform.MaxWorkers = 0
	}
	if err := m.Compression.Normalize(); err != nil {
		return fmt.Errorf("compression: %w", err)
	}
	m.Compression = compressionModel.Merge(compressionModel.Default(), m.Compression)
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
	m.Name = strings.TrimSpace(m.Name)
	m.JwkUrl = strings.TrimSpace(m.JwkUrl)
	if m.JwkUrl == "" {
		return fmt.Errorf("jwk_url: empty")
	}
	return nil
}
