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

// SetJwtCookieNames points every jwt method of auth at the provider's shared
// access cookie and, for a provider the app names an audience for, at the
// app token exchange with the cookies that keep the token. Methods without a
// provider read no cookie.
func (m *App) SetJwtCookieNames(auth *authModel.Auth) {
	for _, method := range auth.Methods {
		if method == nil || method.JWT == nil || method.JWT.Provider == "" {
			continue
		}
		method.JWT.CookieName = constant.JwtCookieName(method.JWT.Provider)
		method.JWT.Exchange = m.jwtExchange(method.JWT.Provider)
	}
}

func (m *App) jwtExchange(provider string) *authModel.JwtExchange {
	audience := m.JwtAudience(provider)
	if audience == "" {
		return nil
	}
	own := authModel.JwtCookie{Name: constant.JwtAppCookieName(provider, m.PathPrefix), Path: m.PathPrefix}
	return &authModel.JwtExchange{
		Audience: audience,
		Cookie:   own,
		Copies: lo.Filter(m.JwtCookieCopies[JwtAudienceKey(provider, audience)], func(v authModel.JwtCookie, _ int) bool {
			return v != own
		}),
	}
}

// JwtAudience is the audience of the app's token at provider, empty when the
// backend gets the shared token.
func (m *App) JwtAudience(provider string) string {
	for i := range m.Jwt {
		if m.Jwt[i].Provider == provider {
			return m.Jwt[i].Audience
		}
	}
	return ""
}

// JwtAudienceKey groups the apps whose tokens are the same: one provider, one
// audience.
func JwtAudienceKey(provider, audience string) string {
	return provider + "\x00" + audience
}

// JwtAudienceCookies collects the app token cookies of apps by provider and
// audience (JwtAudienceKey).
func JwtAudienceCookies(apps []*App) map[string][]authModel.JwtCookie {
	result := make(map[string][]authModel.JwtCookie)
	for _, app := range apps {
		for _, item := range app.Jwt {
			key := JwtAudienceKey(item.Provider, item.Audience)
			result[key] = append(result[key], authModel.JwtCookie{
				Name: constant.JwtAppCookieName(item.Provider, app.PathPrefix),
				Path: app.PathPrefix,
			})
		}
	}
	return result
}
