package model

import (
	"fmt"
	"net/url"
	"strings"

	commonModel "github.com/rendau/ruto/internal/domain/common/model"
	endpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
)

type App struct {
	Id         string                    `json:"id"`
	Active     bool                      `json:"active"`
	PathPrefix string                    `json:"path_prefix"`
	Name       string                    `json:"name"`
	Backend    AppBackend                `json:"backend"`
	Endpoints  []*endpointModel.Endpoint `json:"-"`
}

type AppBackend struct {
	Url       string   `json:"url"`
	ParsedUrl *url.URL `json:"-"`
}

func (m *App) Normalize() error {
	m.PathPrefix = strings.Trim(strings.TrimSpace(m.PathPrefix), "/")
	if m.PathPrefix == "" {
		return fmt.Errorf("path_prefix: empty")
	}
	m.PathPrefix = "/" + m.PathPrefix
	if err := m.Backend.Normalize(); err != nil {
		return fmt.Errorf("backend: %w", err)
	}
	for i := range m.Endpoints {
		if err := m.Endpoints[i].Normalize(); err != nil {
			return fmt.Errorf("endpoints[%d]: %w", i, err)
		}
	}
	return nil
}

func (m *App) String() string {
	return fmt.Sprintf("app{%s}", m.PathPrefix)
}

func (m *AppBackend) Normalize() error {
	var err error
	m.Url = strings.TrimSpace(m.Url)
	if m.Url == "" {
		return fmt.Errorf("url: empty")
	}
	m.ParsedUrl, err = url.Parse(m.Url)
	if err != nil {
		return fmt.Errorf("url: %w", err)
	}
	if m.ParsedUrl.Scheme != "http" && m.ParsedUrl.Scheme != "https" {
		return fmt.Errorf("url: scheme must be http or https")
	}
	if m.ParsedUrl.Host == "" {
		return fmt.Errorf("url: host cannot be empty")
	}
	return nil
}

type ListReq struct {
	commonModel.ListParams

	Active *bool
}
