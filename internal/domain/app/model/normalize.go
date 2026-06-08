package model

import (
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strings"
)

var (
	pathPrefixPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+(?:/[A-Za-z0-9_-]+)*$`)
)

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
	if err := m.Logging.Normalize(); err != nil {
		return fmt.Errorf("logging: %w", err)
	}
	if err := m.Variables.Normalize(); err != nil {
		return fmt.Errorf("variables: %w", err)
	}
	for i := range m.Endpoints {
		if err := m.Endpoints[i].Normalize(); err != nil {
			return fmt.Errorf("endpoints[%d]: %w", i, err)
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
