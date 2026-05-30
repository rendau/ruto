package model

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/samber/lo"

	authModel "github.com/rendau/ruto/internal/domain/auth/model"
	commonModel "github.com/rendau/ruto/internal/domain/common/model"
	endpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
)

var (
	pathPrefixPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+(?:/[A-Za-z0-9_-]+)*$`)
)

type App struct {
	Id         string                    `json:"id"`
	Active     bool                      `json:"active"`
	PathPrefix string                    `json:"path_prefix"`
	Name       string                    `json:"name"`
	Backend    AppBackend                `json:"backend"`
	Auth       authModel.Auth            `json:"auth"`
	Endpoints  []*endpointModel.Endpoint `json:"endpoints"`
}

type AppBackend struct {
	Url              string   `json:"url"`
	ParsedUrl        *url.URL `json:"-"`
	SwaggerUrl       string   `json:"swagger_url"`
	ParsedSwaggerUrl *url.URL `json:"-"`
}

func (m *App) String() string {
	return fmt.Sprintf("app{%s}", m.PathPrefix)
}

func (m *App) Normalize() error {
	m.PathPrefix = strings.Trim(strings.TrimSpace(m.PathPrefix), "/")
	if m.PathPrefix == "" {
		return fmt.Errorf("path_prefix: empty")
	}
	if !pathPrefixPattern.MatchString(m.PathPrefix) {
		return fmt.Errorf("path_prefix: invalid format")
	}
	m.PathPrefix = "/" + m.PathPrefix
	if err := m.Backend.Normalize(); err != nil {
		return fmt.Errorf("backend: %w", err)
	}
	if err := m.Auth.Normalize(); err != nil {
		return fmt.Errorf("auth: %w", err)
	}
	for i := range m.Endpoints {
		if err := m.Endpoints[i].Normalize(); err != nil {
			return fmt.Errorf("endpoints[%d]: %w", i, err)
		}
	}
	return nil
}

func (m *App) ActiveEndpoints() []*endpointModel.Endpoint {
	return lo.FilterMap(m.Endpoints, func(v *endpointModel.Endpoint, _ int) (*endpointModel.Endpoint, bool) {
		return v, v.Active
	})
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

	m.SwaggerUrl = strings.TrimSpace(m.SwaggerUrl)
	m.ParsedSwaggerUrl = nil
	if m.SwaggerUrl != "" {
		m.ParsedSwaggerUrl, err = url.Parse(m.SwaggerUrl)
		if err != nil {
			return fmt.Errorf("swagger_url: %w", err)
		}
		if m.ParsedSwaggerUrl.Scheme != "http" && m.ParsedSwaggerUrl.Scheme != "https" {
			return fmt.Errorf("swagger_url: scheme must be http or https")
		}
		if m.ParsedSwaggerUrl.Host == "" {
			return fmt.Errorf("swagger_url: host cannot be empty")
		}
	}

	return nil
}

func (m *App) GetFullPathForEndpoint(endpointPath string) string {
	if endpointPath == "" {
		return m.PathPrefix
	}
	return m.PathPrefix + "/" + endpointPath
}

type ListReq struct {
	commonModel.ListParams

	Active *bool
}
