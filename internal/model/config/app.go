package config

import (
	"fmt"
	"net/url"
	"strings"
)

type App struct {
	Id               string
	PublicPathPrefix string
	Backend          AppBackend
	Endpoints        []*Endpoint
}

type AppBackend struct {
	UrlStr string
	Url    *url.URL
}

func (m *App) Normalize() error {
	m.Id = strings.TrimSpace(m.Id)
	m.PublicPathPrefix = strings.Trim(strings.TrimSpace(m.PublicPathPrefix), "/")
	if err := validateNotEmpty(m.PublicPathPrefix); err != nil {
		return fmt.Errorf("publicPathPrefix: %w", err)
	}
	m.PublicPathPrefix = "/" + m.PublicPathPrefix
	if err := m.Backend.Normalize(); err != nil {
		return fmt.Errorf("backend: %w", err)
	}
	for _, endpoint := range m.Endpoints {
		if err := endpoint.Normalize(); err != nil {
			return fmt.Errorf("endpoints: %w", err)
		}
	}
	return nil
}

func (m *AppBackend) Normalize() error {
	m.UrlStr = strings.TrimSpace(m.UrlStr)
	err := validateNotEmpty(m.UrlStr)
	if err != nil {
		return fmt.Errorf("url: %w", err)
	}
	m.Url, err = url.Parse(m.UrlStr)
	if err != nil {
		return fmt.Errorf("url: %w", err)
	}
	if m.Url.Scheme != "http" && m.Url.Scheme != "https" {
		return fmt.Errorf("url: scheme must be http or https")
	}
	if m.Url.Host == "" {
		return fmt.Errorf("url: host cannot be empty")
	}
	return nil
}
