package model

import (
	"github.com/samber/lo"

	"github.com/rendau/ruto/internal/constant"
	authModel "github.com/rendau/ruto/internal/domain/auth/model"
	compressionModel "github.com/rendau/ruto/internal/domain/compression/model"
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
	m.SetJwtCookieNames(&ep.Auth)
	ep.Logging = loggingModel.Merge(m.Logging, ep.Logging)
	ep.Compression = compressionModel.Merge(m.Compression, ep.Compression)
	ep.Variables.FillMissing(m.Variables)
}

// SetJwtCookieNames points every jwt method of auth at the cookie its token may
// come from: the app's own cookie for a provider the app brings a login client
// for, the provider's shared cookie otherwise. Methods without a provider read
// no cookie.
func (m *App) SetJwtCookieNames(auth *authModel.Auth) {
	for _, method := range auth.Methods {
		if method == nil || method.JWT == nil || method.JWT.Provider == "" {
			continue
		}
		method.JWT.CookieName = m.JwtCookieName(method.JWT.Provider)
	}
}

// JwtCookieName is the access cookie of provider for requests to this app.
func (m *App) JwtCookieName(provider string) string {
	if m.JwtClient(provider) != nil {
		return constant.JwtAppCookieName(provider, m.PathPrefix)
	}
	return constant.JwtCookieName(provider)
}

// JwtClient is the app's own login client for provider, nil when the app uses
// the provider's default one.
func (m *App) JwtClient(provider string) *authModel.JwtClient {
	for i := range m.Jwt {
		if m.Jwt[i].Provider == provider {
			return &m.Jwt[i].Client
		}
	}
	return nil
}
