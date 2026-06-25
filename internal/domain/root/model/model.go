package model

import (
	"github.com/samber/lo"

	appModel "github.com/rendau/ruto/internal/domain/app/model"
	authModel "github.com/rendau/ruto/internal/domain/auth/model"
	loggingModel "github.com/rendau/ruto/internal/domain/logging/model"
	varsModel "github.com/rendau/ruto/internal/domain/vars/model"
)

type Root struct {
	BaseUrl              string               `json:"base_url"`
	Cors                 RootCors             `json:"cors"`
	Jwt                  []RootJwt            `json:"jwt"`
	Auth                 authModel.Auth       `json:"auth"`
	Logging              loggingModel.Logging `json:"logging"`
	LogOwnResponseErrors bool                 `json:"log_own_response_errors"` // log error responses the gateway returns itself (e.g. auth 401)
	Variables            varsModel.Vars       `json:"variables"`
	Transform            RootTransform        `json:"transform"`

	Apps []*appModel.App `json:"apps"` // not stored in db
}

// RootTransform holds gateway-wide defaults for endpoint transform scripts.
type RootTransform struct {
	// MaxWorkers is the default cap on concurrent goja runtimes per endpoint
	// script. 0 means use the engine default.
	MaxWorkers int `json:"max_workers"`
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

func (m *Root) ActiveApps() []*appModel.App {
	return lo.Filter(m.Apps, func(app *appModel.App, _ int) bool {
		return app.Active
	})
}
