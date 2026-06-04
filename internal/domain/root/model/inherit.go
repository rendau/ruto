package model

import (
	"github.com/samber/lo"

	appModel "github.com/rendau/ruto/internal/domain/app/model"
	authModel "github.com/rendau/ruto/internal/domain/auth/model"
)

func (m *Root) InheritDown() {
	lo.ForEach(m.Apps, m.inheritToApp)
}

func (m *Root) inheritToApp(app *appModel.App, _ int) {
	app.Auth = authModel.Merge(m.Auth, app.Auth)
	app.Variables.FillMissing(m.Variables)
	app.InheritDown()
}
