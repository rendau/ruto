package model

import (
	"fmt"
	"net/url"
	"strings"
)

type App struct {
	Id               string      `json:"id"`
	PublicPathPrefix string      `json:"public_path_prefix"`
	Backend          AppBackend  `json:"backend"`
	Endpoints        []*Endpoint `json:"endpoints"`
}

type AppBackend struct {
	UrlStr string   `json:"url"`
	Url    *url.URL `json:"-"`
}

func (m *App) Normalize() error {
	m.Id = strings.TrimSpace(m.Id)
	m.PublicPathPrefix = strings.Trim(strings.TrimSpace(m.PublicPathPrefix), "/")
	if m.PublicPathPrefix == "" {
		return fmt.Errorf("public_path_prefix: empty")
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
	var err error
	m.UrlStr = strings.TrimSpace(m.UrlStr)
	if m.UrlStr == "" {
		return fmt.Errorf("url: empty")
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
