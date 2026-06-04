package model

import (
	"fmt"
	"strings"

	authModel "github.com/rendau/ruto/internal/domain/auth/model"
	commonModel "github.com/rendau/ruto/internal/domain/common/model"
	variableModel "github.com/rendau/ruto/internal/domain/variable/model"
	varsModel "github.com/rendau/ruto/internal/domain/vars/model"
)

type Type string

const (
	TypeHTTP Type = "http"
	TypeGRPC Type = "grpc"
)

type Endpoint struct {
	Id        string         `json:"id"`
	AppId     string         `json:"app_id"`
	Active    bool           `json:"active"`
	Type      Type           `json:"type"`
	Http      Http           `json:"http"`
	Grpc      Grpc           `json:"grpc"`
	Backend   Backend        `json:"backend"`
	Auth      authModel.Auth `json:"auth"`
	Variables varsModel.Vars `json:"variables"`
}

type Http struct {
	Method string `json:"method"`
	Path   string `json:"path"`
}

type Grpc struct {
	Service string `json:"service"`
	Method  string `json:"method"`
	Path    string `json:"path"`
}

type Backend struct {
	CustomPath  string            `json:"custom_path"`
	Headers     map[string]string `json:"headers"`
	QueryParams map[string]string `json:"query_params"`
}

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
	var err error
	m.Variables, err = variableModel.NormalizeList(m.Variables)
	if err != nil {
		return fmt.Errorf("variables: %w", err)
	}

	return nil
}

func (m *Backend) Normalize() error {
	m.CustomPath = strings.TrimPrefix(strings.TrimSpace(m.CustomPath), "/")
	m.Headers = normalizeStringMap(m.Headers)
	m.QueryParams = normalizeStringMap(m.QueryParams)
	return nil
}

func (m *Http) Normalize() error {
	m.Method = strings.ToUpper(strings.TrimSpace(m.Method))
	m.Path = strings.Trim(strings.TrimSpace(m.Path), "/")
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
