package model

import (
	"github.com/samber/lo"

	appModel "github.com/rendau/ruto/internal/domain/app/model"
	authModel "github.com/rendau/ruto/internal/domain/auth/model"
	compressionModel "github.com/rendau/ruto/internal/domain/compression/model"
	loggingModel "github.com/rendau/ruto/internal/domain/logging/model"
	rateLimitModel "github.com/rendau/ruto/internal/domain/ratelimit/model"
)

func (m *Root) InheritDown() {
	m.RateLimit.SetScope(rateLimitModel.ScopeRoot)
	lo.ForEach(m.Apps, m.inheritToApp)
}

func (m *Root) inheritToApp(app *appModel.App, _ int) {
	app.Auth = authModel.Merge(m.Auth, app.Auth)
	app.Logging = loggingModel.Merge(m.Logging, app.Logging)
	app.Compression = compressionModel.Merge(m.Compression, app.Compression)
	app.RateLimit.SetScope(rateLimitModel.ScopeApp(app.Id))
	app.RateLimit = rateLimitModel.Merge(m.RateLimit, app.RateLimit)
	app.Variables.FillMissing(m.Variables)
	app.InheritDown()
}
