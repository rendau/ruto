package model

import (
	"github.com/samber/lo"

	authModel "github.com/rendau/ruto/internal/domain/auth/model"
	endpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
	loggingModel "github.com/rendau/ruto/internal/domain/logging/model"
)

func (m *App) InheritDown() {
	lo.ForEach(m.Endpoints, m.inheritToEndpoint)
}

func (m *App) inheritToEndpoint(ep *endpointModel.Endpoint, _ int) {
	ep.Backend.Headers.FillMissing(m.Backend.Headers)
	ep.Backend.QueryParams.FillMissing(m.Backend.QueryParams)
	ep.Auth = authModel.Merge(m.Auth, ep.Auth)
	ep.Logging = loggingModel.Merge(m.Logging, ep.Logging)
	ep.Variables.FillMissing(m.Variables)
}
