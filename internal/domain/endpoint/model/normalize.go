package model

import (
	"fmt"
	"strings"
)

func (m *Endpoint) Normalize() error {
	m.Type = normalizeType(m.Type)
	if m.Type == "" {
		return fmt.Errorf("type: invalid")
	}

	switch m.Type {
	case TypeHTTP:
		m.Grpc = Grpc{}
		if err := m.Http.Normalize(); err != nil {
			return fmt.Errorf("http: %w", err)
		}
	case TypeGRPC:
		m.Http = Http{}
		if err := m.Grpc.Normalize(); err != nil {
			return fmt.Errorf("grpc: %w", err)
		}
	}

	if err := m.Backend.Normalize(); err != nil {
		return fmt.Errorf("backend: %w", err)
	}

	if err := m.Auth.Normalize(); err != nil {
		return fmt.Errorf("auth: %w", err)
	}

	if err := m.Variables.Normalize(); err != nil {
		return fmt.Errorf("variables: %w", err)
	}

	return nil
}

func (m *Http) Normalize() error {
	m.Method = strings.ToUpper(strings.TrimSpace(m.Method))
	if m.Method == "" {
		return fmt.Errorf("method: empty")
	}

	m.Path = strings.Trim(strings.TrimSpace(m.Path), "/")
	if strings.Contains(m.Path, "*") {
		return fmt.Errorf("path: wildcard '*' is not allowed")
	}

	return nil
}

func (m *Grpc) Normalize() error {
	m.Service = strings.TrimSpace(m.Service)
	if m.Service == "" {
		return fmt.Errorf("service: empty")
	}
	m.Method = strings.TrimSpace(m.Method)
	if m.Method == "" {
		return fmt.Errorf("method: empty")
	}
	m.Path = strings.TrimSpace(m.Path)
	if m.Path == "" {
		return fmt.Errorf("path: empty")
	}
	if !strings.HasPrefix(m.Path, "/") {
		return fmt.Errorf("path: must start with '/'")
	}
	parts := strings.Split(strings.TrimPrefix(m.Path, "/"), "/")
	if len(parts) != 2 {
		return fmt.Errorf("path: invalid format")
	}
	return nil
}

func (m *Backend) Normalize() error {
	m.CustomPath = strings.TrimPrefix(strings.TrimSpace(m.CustomPath), "/")
	if err := m.Headers.Normalize(); err != nil {
		return fmt.Errorf("headers: %w", err)
	}
	if err := m.QueryParams.Normalize(); err != nil {
		return fmt.Errorf("query_params: %w", err)
	}
	return nil
}

func normalizeType(v Type) Type {
	switch Type(strings.ToLower(strings.TrimSpace(string(v)))) {
	case "", TypeHTTP:
		return TypeHTTP
	case TypeGRPC:
		return TypeGRPC
	default:
		return ""
	}
}
