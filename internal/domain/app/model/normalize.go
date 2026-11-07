package model

import (
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strings"

	"github.com/rendau/ruto/internal/constant"
)

var (
	pathPrefixPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+(?:/[A-Za-z0-9_-]+)*$`)
)

func (m *App) Normalize() error {
	m.Group = strings.TrimSpace(m.Group)
	m.PathPrefix = strings.Trim(strings.TrimSpace(m.PathPrefix), "/")
	if m.PathPrefix == "" {
		return fmt.Errorf("path_prefix: empty")
	}
	if !pathPrefixPattern.MatchString(m.PathPrefix) {
		return fmt.Errorf("path_prefix: invalid format")
	}
	if first, _, _ := strings.Cut(m.PathPrefix, "/"); first == constant.AuthRoutePrefix {
		return fmt.Errorf("path_prefix: /%s is reserved for the gateway login routes", constant.AuthRoutePrefix)
	}
	m.PathPrefix = "/" + m.PathPrefix
	if err := m.Backend.Normalize(); err != nil {
		return fmt.Errorf("backend: %w", err)
	}
	if err := m.Auth.Normalize(); err != nil {
		return fmt.Errorf("auth: %w", err)
	}
	if m.Jwt == nil {
		m.Jwt = []AppJwt{}
	}
	providers := make(map[string]struct{}, len(m.Jwt))
	for i := range m.Jwt {
		if err := m.Jwt[i].Normalize(); err != nil {
			return fmt.Errorf("jwt[%d]: %w", i, err)
		}
		if _, ok := providers[m.Jwt[i].Provider]; ok {
			return fmt.Errorf("jwt[%d]: provider: duplicate %q", i, m.Jwt[i].Provider)
		}
		providers[m.Jwt[i].Provider] = struct{}{}
	}
	if err := m.Logging.Normalize(); err != nil {
		return fmt.Errorf("logging: %w", err)
	}
	if err := m.Variables.Normalize(); err != nil {
		return fmt.Errorf("variables: %w", err)
	}
	if err := m.Compression.Normalize(); err != nil {
		return fmt.Errorf("compression: %w", err)
	}
	for i := range m.Endpoints {
		if err := m.Endpoints[i].Normalize(); err != nil {
			return fmt.Errorf("endpoints[%d]: %w", i, err)
		}
		if !m.AllowWildcard && m.Endpoints[i].Http.HasWildcard() {
			return fmt.Errorf("endpoints[%d]: http: path: wildcard '*' is not allowed for this app", i)
		}
	}
	return nil
}

func (m *Backend) Normalize() error {
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

	m.GrpcUrl = strings.TrimSpace(m.GrpcUrl)
	if m.GrpcUrl != "" {
		if err = validateGrpcTarget(m.GrpcUrl); err != nil {
			return fmt.Errorf("grpc_url: %w", err)
		}
	}

	if err = m.Headers.Normalize(); err != nil {
		return fmt.Errorf("headers: %w", err)
	}
	if err = m.QueryParams.Normalize(); err != nil {
		return fmt.Errorf("query_params: %w", err)
	}

	return nil
}

func validateGrpcTarget(target string) error {
	if strings.Contains(target, "://") {
		u, err := url.Parse(target)
		if err != nil {
			return err
		}

		switch u.Scheme {
		case "dns", "unix", "passthrough":
			return nil
		default:
			return fmt.Errorf("unsupported scheme %q", u.Scheme)
		}
	}

	_, _, err := net.SplitHostPort(target)
	return err
}

func (m *AppJwt) Normalize() error {
	m.Provider = strings.TrimSpace(m.Provider)
	if m.Provider == "" {
		return fmt.Errorf("provider: empty")
	}
	if err := m.Client.Normalize(); err != nil {
		return fmt.Errorf("client: %w", err)
	}
	return nil
}
