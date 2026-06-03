package model

import (
	"fmt"
	"strings"

	"github.com/samber/lo"

	appModel "github.com/rendau/ruto/internal/domain/app/model"
	authModel "github.com/rendau/ruto/internal/domain/auth/model"
	endpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
	variableModel "github.com/rendau/ruto/internal/domain/variable/model"
)

type Root struct {
	BaseUrl   string                   `json:"base_url"`
	Cors      RootCors                 `json:"cors"`
	Jwt       []RootJwt                `json:"jwt"`
	Auth      authModel.Auth           `json:"auth"`
	Variables []variableModel.Variable `json:"variables"`
	Apps      []*appModel.App          `json:"apps"`
}

type RootCors struct {
	Enabled          bool     `json:"enabled"`
	AllowCredentials bool     `json:"allow_credentials"`
	MaxAge           string   `json:"max_age"`
	AllowOrigins     []string `json:"allow_origins"`
	AllowMethods     []string `json:"allow_methods"`
	AllowHeaders     []string `json:"allow_headers"`
}

type RootJwt struct {
	JwkUrl string `json:"jwk_url"`
}

func (m *Root) String() string {
	return fmt.Sprintf("root{%s}", m.BaseUrl)
}

func (m *Root) Normalize() error {
	m.BaseUrl = strings.TrimRight(strings.TrimSpace(m.BaseUrl), "/")
	if err := m.Cors.Normalize(); err != nil {
		return fmt.Errorf("cors: %w", err)
	}
	for i := range m.Jwt {
		if err := m.Jwt[i].Normalize(); err != nil {
			return fmt.Errorf("jwt[%d]: %w", i, err)
		}
	}
	if err := m.Auth.Normalize(); err != nil {
		return fmt.Errorf("auth: %w", err)
	}
	var err error
	m.Variables, err = variableModel.NormalizeList(m.Variables)
	if err != nil {
		return fmt.Errorf("variables: %w", err)
	}
	for i := range m.Apps {
		if err := m.Apps[i].Normalize(); err != nil {
			return fmt.Errorf("apps[%d]: %w", i, err)
		}
	}
	return nil
}

func (m *Root) ActiveApps() []*appModel.App {
	return lo.FilterMap(m.Apps, func(v *appModel.App, _ int) (*appModel.App, bool) {
		return v, v.Active
	})
}

func (m *Root) EffectiveVariables(app *appModel.App, endpoint *endpointModel.Endpoint) ([]variableModel.Variable, error) {
	result := m.Variables
	if app != nil {
		result = variableModel.Merge(result, app.Variables)
	}
	if endpoint != nil {
		result = variableModel.Merge(result, endpoint.Variables)
	}

	if _, err := variableModel.Resolve(result); err != nil {
		return nil, err
	}
	return result, nil
}

func (m *RootCors) Normalize() error {
	m.MaxAge = strings.TrimSpace(m.MaxAge)
	m.AllowOrigins = lo.FilterMap(m.AllowOrigins, func(v string, _ int) (string, bool) {
		v = strings.TrimSpace(v)
		return v, v != ""
	})
	m.AllowMethods = lo.FilterMap(m.AllowMethods, func(v string, _ int) (string, bool) {
		v = strings.ToUpper(strings.TrimSpace(v))
		return v, v != ""
	})
	m.AllowHeaders = lo.FilterMap(m.AllowHeaders, func(v string, _ int) (string, bool) {
		v = strings.TrimSpace(v)
		return v, v != ""
	})
	return nil
}

func (m *RootJwt) Normalize() error {
	m.JwkUrl = strings.TrimSpace(m.JwkUrl)
	if m.JwkUrl == "" {
		return fmt.Errorf("jwk_url: empty")
	}
	return nil
}

func NewEmpty() *Root {
	return &Root{
		Cors: RootCors{
			Enabled:          false,
			AllowCredentials: false,
			MaxAge:           "864000",
			AllowOrigins:     []string{"*"},
			AllowMethods:     []string{"*"},
			AllowHeaders:     []string{"*"},
		},
		Jwt:  []RootJwt{},
		Apps: []*appModel.App{},
	}
}
