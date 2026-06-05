package model

import (
	"net/url"

	"github.com/samber/lo"

	authModel "github.com/rendau/ruto/internal/domain/auth/model"
	commonModel "github.com/rendau/ruto/internal/domain/common/model"
	endpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
	varsModel "github.com/rendau/ruto/internal/domain/vars/model"
)

type App struct {
	Id                 string         `json:"id"`
	Active             bool           `json:"active"`
	ExcludeFromMetrics bool           `json:"exclude_from_metrics"`
	PathPrefix         string         `json:"path_prefix"`
	Name               string         `json:"name"`
	Backend            Backend        `json:"backend"`
	Auth               authModel.Auth `json:"auth"`
	Variables          varsModel.Vars `json:"variables"`

	Endpoints []*endpointModel.Endpoint `json:"endpoints"` // not stored in db
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
