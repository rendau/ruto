package model

import (
	"github.com/samber/lo"

	endpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
	varsModel "github.com/rendau/ruto/internal/domain/vars/model"
)

// Redacted returns a copy of the app with all secret material masked (auth
// secrets, variable values, backend header/query-param values). It is used when
// an app is returned to a user that may view but not manage it. The receiver
// and its nested maps/slices are left untouched.
func (m *App) Redacted() *App {
	result := *m

	result.Auth = m.Auth.Redacted()
	result.Variables = varsModel.RedactedValues(m.Variables)
	result.Backend.Headers = varsModel.RedactedValues(m.Backend.Headers)
	result.Backend.QueryParams = varsModel.RedactedValues(m.Backend.QueryParams)

	if len(m.Endpoints) > 0 {
		result.Endpoints = lo.Map(m.Endpoints, func(ep *endpointModel.Endpoint, _ int) *endpointModel.Endpoint {
			return ep.Redacted()
		})
	}

	return &result
}
