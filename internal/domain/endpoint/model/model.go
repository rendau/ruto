package model

import (
	"fmt"
	"strings"

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
}

type Backend struct {
	CustomPath string `json:"custom_path"`
}

func (m *Endpoint) String() string {
	return fmt.Sprintf("endpoint{%s %s}", m.Method, m.Path)
}

func (m *Endpoint) Normalize() error {
	m.Method = strings.ToUpper(strings.TrimSpace(m.Method))
	if m.Method == "" {
		return fmt.Errorf("method: empty")
	}

	m.Path = strings.Trim(strings.TrimSpace(m.Path), "/")
	if m.Path == "" {
		return fmt.Errorf("path: empty")
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
	return nil
}

type ListReq struct {
	commonModel.ListParams

	AppId  *string
	Active *bool
}
