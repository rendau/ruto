package model

import (
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/samber/lo"

	"github.com/rendau/ruto/internal/constant"
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
	// custom_path replaces the whole backend path, so the wildcard tail would be lost.
	if m.Http.HasWildcard() && m.Backend.CustomPath != "" {
		return fmt.Errorf("backend: custom_path: not allowed with wildcard path")
	}
	if !m.Http.HasWildcard() {
		m.TrackPaths = false
	}
	// A wildcard already catches the path with a trailing slash.
	if m.Type != TypeHTTP || m.Http.HasWildcard() {
		m.AllowTrailingSlash = false
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

	m.Transform.Normalize()

	if m.Type != TypeHTTP {
		m.Cache = Cache{}
	}
	if err := m.Cache.Normalize(); err != nil {
		return fmt.Errorf("cache: %w", err)
	}

	return nil
}

func (m *Cache) Normalize() error {
	if err := m.Server.Normalize(); err != nil {
		return fmt.Errorf("server: %w", err)
	}
	if err := m.Browser.Normalize(); err != nil {
		return fmt.Errorf("browser: %w", err)
	}
	return nil
}

func (m *CacheServer) Normalize() error {
	if m.Ttl < 0 {
		return fmt.Errorf("ttl: must be >= 0")
	}
	if m.Enabled && m.Ttl == 0 {
		return fmt.Errorf("ttl: must be > 0")
	}
	if m.MaxBodySize < 0 {
		return fmt.Errorf("max_body_size: must be >= 0")
	}

	m.VaryHeaders = lo.Uniq(lo.FilterMap(m.VaryHeaders, func(v string, _ int) (string, bool) {
		v = http.CanonicalHeaderKey(strings.TrimSpace(v))
		return v, v != ""
	}))
	slices.Sort(m.VaryHeaders)

	return nil
}

func (m *CacheServer) MaxBodySizeOrDefault() int {
	if m.MaxBodySize <= 0 {
		return constant.DefaultCacheMaxBodySize
	}
	return m.MaxBodySize
}

func (m *CacheBrowser) Normalize() error {
	m.Scope = strings.ToLower(strings.TrimSpace(m.Scope))
	if m.Scope == "" {
		m.Scope = constant.CacheScopePrivate
	}
	if !constant.CacheScopeIsValid(m.Scope) {
		return fmt.Errorf("scope: is invalid")
	}
	if m.MaxAge < 0 {
		return fmt.Errorf("max_age: must be >= 0")
	}
	return nil
}

// HeaderValue is the Cache-Control value sent to the client.
func (m *CacheBrowser) HeaderValue() string {
	if m.Scope == constant.CacheScopeNoStore {
		return constant.CacheScopeNoStore
	}
	return fmt.Sprintf("%s, max-age=%d", m.Scope, m.MaxAge)
}

func (m *Transform) Normalize() {
	m.Request = strings.TrimSpace(m.Request)
	m.Response = strings.TrimSpace(m.Response)
	if m.MaxWorkers < 0 {
		m.MaxWorkers = 0
	}
}

func (m *Http) Normalize() error {
	m.Method = strings.ToUpper(strings.TrimSpace(m.Method))
	if m.Method == "" {
		return fmt.Errorf("method: empty")
	}

	m.Path = strings.Trim(strings.TrimSpace(m.Path), "/")
	// The router accepts '*' only as the whole last segment and panics otherwise.
	if strings.Contains(m.Path, "*") && m.Path != "*" &&
		(!strings.HasSuffix(m.Path, "/*") || strings.Count(m.Path, "*") != 1) {
		return fmt.Errorf("path: wildcard '*' is allowed only as the last segment")
	}

	return nil
}

func (m *Http) HasWildcard() bool {
	return strings.HasSuffix(m.Path, "*")
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
