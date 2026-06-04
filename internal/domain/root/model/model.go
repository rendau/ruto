package model

import (
	"fmt"
	"strings"

	"github.com/samber/lo"

	appModel "github.com/rendau/ruto/internal/domain/app/model"
	authModel "github.com/rendau/ruto/internal/domain/auth/model"
	varsModel "github.com/rendau/ruto/internal/domain/vars/model"
)

type Root struct {
	BaseUrl   string         `json:"base_url"`
	Cors      RootCors       `json:"cors"`
	Jwt       []RootJwt      `json:"jwt"`
	Auth      authModel.Auth `json:"auth"`
	Variables varsModel.Vars `json:"variables"`

	Apps                   []*appModel.App `json:"apps"`                     // not stored in db
	MergedApps             []*appModel.App `json:"merged_apps"`              // not stored in db
	InterpolatedMergedApps []*appModel.App `json:"interpolated_merged_apps"` // not stored in db
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
	if err := m.Variables.Normalize(); err != nil {
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
