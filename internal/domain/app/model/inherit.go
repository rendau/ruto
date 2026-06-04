package model

import (
	"github.com/samber/lo"

	authModel "github.com/rendau/ruto/internal/domain/auth/model"
	endpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
)

func (m *App) InheritDown() {
	lo.ForEach(m.Endpoints, m.inheritToEndpoint)
}

func (m *App) inheritToEndpoint(ep *endpointModel.Endpoint, _ int) {
	ep.Backend.Headers.FillMissing(m.Backend.Headers)
	ep.Backend.QueryParams.FillMissing(m.Backend.QueryParams)
	ep.Auth = authModel.Merge(m.Auth, ep.Auth)
	ep.Variables.FillMissing(m.Variables)
}
