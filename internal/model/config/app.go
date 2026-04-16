package config

import (
	"fmt"
	"strings"
)

type App struct {
	Id         string
	PublicPath string
	Backend    AppBackend
	Endpoints  []Endpoint
}

type AppBackend struct {
	Host string
	Path string
}

func (m *App) Normalize() {
	m.Id = strings.TrimSpace(m.Id)
	m.PublicPath = strings.TrimPrefix(strings.TrimSpace(m.PublicPath), "/")

	m.Backend.Normalize()
	for i := range m.Endpoints {
		m.Endpoints[i].Normalize()
	}
}

func (m *AppBackend) Normalize() {
	m.Host = strings.TrimSpace(m.Host)
	m.Path = strings.TrimPrefix(strings.TrimSpace(m.Path), "/")
}

func (m *App) Validate() error {
	if err := validateNotEmpty(m.Id); err != nil {
		return fmt.Errorf("id: %w", err)
	}
	if err := validateNotEmpty(m.PublicPath); err != nil {
		return fmt.Errorf("public_path: %w", err)
	}
	if err := m.Backend.Validate(); err != nil {
		return fmt.Errorf("backend: %w", err)
	}
	for i := range m.Endpoints {
		if err := m.Endpoints[i].Validate(); err != nil {
			return fmt.Errorf("endpoints[%d]: %w", i, err)
		}
	}
	return nil
}

func (m *AppBackend) Validate() error {
	if err := validateNotEmpty(m.Host); err != nil {
		return fmt.Errorf("host: %w", err)
	}
	if err := validateNotEmpty(m.Path); err != nil {
		return fmt.Errorf("path: %w", err)
	}
	return nil
}
