package model

import (
	"github.com/samber/lo"

	authModel "github.com/rendau/ruto/internal/domain/auth/model"
	compressionModel "github.com/rendau/ruto/internal/domain/compression/model"
	endpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
	loggingModel "github.com/rendau/ruto/internal/domain/logging/model"
	rateLimitModel "github.com/rendau/ruto/internal/domain/ratelimit/model"
)

func (m *App) InheritDown() {
	m.RateLimit.SetScope(rateLimitModel.ScopeApp(m.Id))
	lo.ForEach(m.Endpoints, m.inheritToEndpoint)
}

func (m *App) inheritToEndpoint(ep *endpointModel.Endpoint, _ int) {
	ep.Backend.Headers.FillMissing(m.Backend.Headers)
	ep.Backend.QueryParams.FillMissing(m.Backend.QueryParams)
	ep.Auth = authModel.Merge(m.Auth, ep.Auth)
	ep.Logging = loggingModel.Merge(m.Logging, ep.Logging)
	ep.Compression = compressionModel.Merge(m.Compression, ep.Compression)
	ep.RateLimit.SetScope(rateLimitModel.ScopeEndpoint(ep.Id))
	ep.RateLimit = rateLimitModel.Merge(m.RateLimit, ep.RateLimit)
	ep.Variables.FillMissing(m.Variables)
}
