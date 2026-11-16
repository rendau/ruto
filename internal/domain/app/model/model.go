package model

import (
	"net/url"

	"github.com/samber/lo"

	authModel "github.com/rendau/ruto/internal/domain/auth/model"
	commonModel "github.com/rendau/ruto/internal/domain/common/model"
	compressionModel "github.com/rendau/ruto/internal/domain/compression/model"
	endpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
	loggingModel "github.com/rendau/ruto/internal/domain/logging/model"
	varsModel "github.com/rendau/ruto/internal/domain/vars/model"
)

type App struct {
	Id                 string                       `json:"id"`
	Active             bool                         `json:"active"`
	ExcludeFromMetrics bool                         `json:"exclude_from_metrics"`
	AllowWildcard      bool                         `json:"allow_wildcard"` // endpoints may use a trailing '*' in http.path
	PathPrefix         string                       `json:"path_prefix"`
	Name               string                       `json:"name"`
	Group              string                       `json:"group"` // free-form label used by the admin to group apps visually
	Backend            Backend                      `json:"backend"`
	Auth               authModel.Auth               `json:"auth"`
	Logging            loggingModel.Logging         `json:"logging"`
	Variables          varsModel.Vars               `json:"variables"`
	Compression        compressionModel.Compression `json:"compression"`
	// Jwt names, per provider, the audience of the app's own token: the
	// gateway exchanges the shared login token for a token with that audience
	// and hands it to the backend. Without an entry the backend gets the
	// shared token.
	Jwt []AppJwt `json:"jwt"`

	Endpoints []*endpointModel.Endpoint `json:"endpoints"` // not stored in db

	// JwtCookieCopies are the app token cookies of all apps of the snapshot
	// by provider and audience (JwtAudienceKey), so one exchange fills the
	// cookies of every app with the same audience. Set by the root before it
	// inherits down.
	JwtCookieCopies map[string][]authModel.JwtCookie `json:"-"`
}

// AppJwt is the audience the app's token is exchanged for at Provider (RFC
// 8693): in Keycloak the client_id of the client that holds the app's roles.
type AppJwt struct {
	Provider string `json:"provider"`
	Audience string `json:"audience"`
}

type Backend struct {
	Url              string         `json:"url"`
	ParsedUrl        *url.URL       `json:"-"`
	SwaggerUrl       string         `json:"swagger_url"`
	ParsedSwaggerUrl *url.URL       `json:"-"`
	GrpcUrl          string         `json:"grpc_url"`
	Headers          varsModel.Vars `json:"headers"`
	QueryParams      varsModel.Vars `json:"query_params"`
}

type ListReq struct {
	commonModel.ListParams

	Active    *bool
	NameEqCI  *string
	ExcludeID *string
	IdsIn     *[]string // if set, only return apps with these IDs
}

func (m *App) ActiveEndpoints() []*endpointModel.Endpoint {
	return lo.Filter(m.Endpoints, func(ep *endpointModel.Endpoint, _ int) bool {
		return ep.Active
	})
}
