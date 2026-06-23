package model

import (
	varsModel "github.com/rendau/ruto/internal/domain/vars/model"
)

// Redacted returns a copy of the endpoint with all secret material masked (auth
// secrets, variable values, backend header/query-param values). It is used when
// an endpoint is returned to a user that may view but not manage its app. The
// receiver and its nested maps are left untouched.
func (m *Endpoint) Redacted() *Endpoint {
	result := *m

	result.Auth = m.Auth.Redacted()
	result.Variables = varsModel.RedactedValues(m.Variables)
	result.Backend.Headers = varsModel.RedactedValues(m.Backend.Headers)
	result.Backend.QueryParams = varsModel.RedactedValues(m.Backend.QueryParams)

	return &result
}
