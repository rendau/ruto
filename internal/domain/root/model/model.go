package model

import (
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

	Apps []*appModel.App `json:"apps"` // not stored in db
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
