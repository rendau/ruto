package model

import (
	"fmt"
	"strings"

	"github.com/samber/lo"

	authModel "github.com/rendau/ruto/internal/domain/auth/model"
	commonModel "github.com/rendau/ruto/internal/domain/common/model"
)

type Endpoint struct {
	Id      string         `json:"id"`
	AppId   string         `json:"app_id"`
	Active  bool           `json:"active"`
	Method  string         `json:"method"`
	Path    string         `json:"path"`
	Backend Backend        `json:"backend"`
	Auth    authModel.Auth `json:"auth"`
	Type    Type           `json:"type"`
	Grpc    Grpc           `json:"grpc"`
}

type Backend struct {
	CustomPath  string            `json:"custom_path"`
	Headers     map[string]string `json:"headers"`
	QueryParams map[string]string `json:"query_params"`
}

type Type string

const (
	TypeHTTP Type = "http"
	TypeGRPC Type = "grpc"
)

type Grpc struct {
	Service string `json:"service"`
	Method  string `json:"method"`
	Path    string `json:"path"`
}

func (m *Endpoint) String() string {
	return fmt.Sprintf("endpoint{%s %s}", m.Method, m.Path)
}

func (m *Endpoint) Normalize() error {
	m.Type = normalizeType(m.Type)
	if m.Type == "" {
		return fmt.Errorf("type: invalid")
	}

	if m.Type == TypeGRPC {
		if err := m.Grpc.Normalize(); err != nil {
			return fmt.Errorf("grpc: %w", err)
		}
		m.Method = "GRPC"
		m.Path = strings.TrimPrefix(m.Grpc.Path, "/")
	} else {
		m.Grpc = Grpc{}
		m.Method = strings.ToUpper(strings.TrimSpace(m.Method))
		m.Path = strings.Trim(strings.TrimSpace(m.Path), "/")
	}

	if m.Method == "" {
		return fmt.Errorf("method: empty")
	}

	if strings.Contains(m.Path, "*") {
		return fmt.Errorf("path: wildcard '*' is not allowed")
	}

	if err := m.Backend.Normalize(); err != nil {
		return fmt.Errorf("backend: %w", err)
	}
	if err := m.Auth.Normalize(); err != nil {
		return fmt.Errorf("auth: %w", err)
	}

	return nil
}

func (m *Backend) Normalize() error {
	m.CustomPath = strings.TrimPrefix(strings.TrimSpace(m.CustomPath), "/")
	m.Headers = normalizeStringMap(m.Headers)
	m.QueryParams = normalizeStringMap(m.QueryParams)
	return nil
}

func normalizeStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	result := lo.PickBy(
		lo.MapEntries(values, func(key, value string) (string, string) {
			return strings.TrimSpace(key), strings.TrimSpace(value)
		}),
		func(key, _ string) bool {
			return key != ""
		},
	)
	if len(result) == 0 {
		return nil
	}
	return result
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

type ListReq struct {
	commonModel.ListParams

	AppId  *string
	Active *bool
}
