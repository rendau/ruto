package model

import (
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/samber/lo"

	authModel "github.com/rendau/ruto/internal/domain/auth/model"
	commonModel "github.com/rendau/ruto/internal/domain/common/model"
	endpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
	variableModel "github.com/rendau/ruto/internal/domain/variable/model"
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
	Variables  []variableModel.Variable  `json:"variables"`
	Endpoints  []*endpointModel.Endpoint `json:"endpoints"`
}

type AppBackend struct {
	Url              string            `json:"url"`
	ParsedUrl        *url.URL          `json:"-"`
	SwaggerUrl       string            `json:"swagger_url"`
	ParsedSwaggerUrl *url.URL          `json:"-"`
	GrpcPort         int               `json:"grpc_port"`
	Headers          map[string]string `json:"headers"`
	QueryParams      map[string]string `json:"query_params"`
}

type BackendRequestParams struct {
	Headers     map[string]string
	QueryParams map[string]string
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
	var err error
	m.Variables, err = variableModel.NormalizeList(m.Variables)
	if err != nil {
		return fmt.Errorf("variables: %w", err)
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

func (m *App) BackendRequestParams(endpoint *endpointModel.Endpoint) BackendRequestParams {
	return BackendRequestParams{
		Headers:     mergeStringMaps(m.Backend.Headers, endpoint.Backend.Headers),
		QueryParams: mergeStringMaps(m.Backend.QueryParams, endpoint.Backend.QueryParams),
	}
}

func (m *App) BackendRequestParamsWithVariables(endpoint *endpointModel.Endpoint, variables []variableModel.Variable) (BackendRequestParams, error) {
	scope, err := variableModel.Resolve(variables)
	if err != nil {
		return BackendRequestParams{}, err
	}

	params := m.BackendRequestParams(endpoint)
	params.Headers, err = variableModel.InterpolateMap(params.Headers, scope)
	if err != nil {
		return BackendRequestParams{}, fmt.Errorf("headers: %w", err)
	}
	params.QueryParams, err = variableModel.InterpolateMap(params.QueryParams, scope)
	if err != nil {
		return BackendRequestParams{}, fmt.Errorf("query_params: %w", err)
	}

	return params, nil
}

func (m *App) GrpcAddress() string {
	return m.Backend.GrpcAddress()
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

	if m.GrpcPort < 0 || m.GrpcPort > 65535 {
		return fmt.Errorf("grpc_port: invalid")
	}
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

func mergeStringMaps(base, override map[string]string) map[string]string {
	if len(base) == 0 {
		return override
	}
	if len(override) == 0 {
		return base
	}
	return lo.Assign(base, override)
}

func (m *AppBackend) GrpcAddress() string {
	if m.GrpcPort <= 0 {
		return ""
	}

	parsedURL := m.ParsedUrl
	if parsedURL == nil {
		backend := *m
		if err := backend.Normalize(); err != nil {
			return ""
		}
		parsedURL = backend.ParsedUrl
	}

	host := parsedURL.Hostname()
	if host == "" {
		return ""
	}

	return net.JoinHostPort(host, strconv.Itoa(m.GrpcPort))
}

func (m *App) GetFullPathForEndpoint(endpointPath string) string {
	if endpointPath == "" {
		return m.PathPrefix
	}
	return m.PathPrefix + "/" + endpointPath
}

type ListReq struct {
	commonModel.ListParams

	Active    *bool
	NameEqCI  *string
	ExcludeID *string
}
