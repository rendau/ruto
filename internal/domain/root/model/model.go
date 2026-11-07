package model

import (
	"github.com/samber/lo"

	appModel "github.com/rendau/ruto/internal/domain/app/model"
	authModel "github.com/rendau/ruto/internal/domain/auth/model"
	compressionModel "github.com/rendau/ruto/internal/domain/compression/model"
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
	// Compression is always complete on a root: Normalize fills the empty
	// fields with the defaults.
	Compression compressionModel.Compression `json:"compression"`

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

// RootJwt is a JWT provider: a JWKS endpoint whose keys the gateway trusts.
// Name is what auth methods refer to; a provider without a name only serves
// methods that pin no provider.
//
// With Issuer set the provider is an OpenID Connect server: its endpoints come
// from discovery (JwkUrl may then be left empty) and the gateway can log users
// in on behalf of Client, the default login client. An app may bring its own
// client instead (App.Jwt).
type RootJwt struct {
	Name   string `json:"name"`
	JwkUrl string `json:"jwk_url"`
	// Issuer is the public address of the OpenID provider, the "iss" of its
	// tokens and the base of its discovery document.
	Issuer string `json:"issuer"`
	// InternalUrl, when set, replaces the Issuer prefix of the endpoints the
	// gateway itself calls (token, keys): the way in from inside the cluster.
	InternalUrl string `json:"internal_url"`
	// PublicUrl is the gateway address the provider sends the browser back to;
	// empty means Root.BaseUrl.
	PublicUrl string               `json:"public_url"`
	Client    *authModel.JwtClient `json:"client,omitempty"`
	// TransparentRefresh lets the gateway renew an expired session inside an
	// app request instead of answering 401. The refresh cookie then travels
	// with every request (Path=/), the gateway keeps it from the backends.
	TransparentRefresh bool `json:"transparent_refresh"`
}

// LoginEnabled reports whether the gateway can log users in through this
// provider at all (an app client still needs it).
func (m *RootJwt) LoginEnabled() bool {
	return m.Issuer != ""
}

func NewEmpty() *Root {
	return &Root{
		Cors: RootCors{
			Enabled:          false,
			AllowCredentials: false,
			MaxAge:           "240h",
			AllowOrigins:     []string{"*"},
			AllowMethods:     []string{"*"},
			AllowHeaders:     []string{"*"},
		},
		Jwt:         []RootJwt{},
		Compression: compressionModel.Default(),
		Apps:        []*appModel.App{},
	}
}

func (m *Root) ActiveApps() []*appModel.App {
	return lo.Filter(m.Apps, func(app *appModel.App, _ int) bool {
		return app.Active
	})
}
