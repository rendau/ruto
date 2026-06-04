package model

import (
	"github.com/samber/lo"

	appModel "github.com/rendau/ruto/internal/domain/app/model"
)

func (m *Root) Interpolate() {

	lo.ForEach(m.Apps, m.inheritToApp)
}

func (m *Root) interpolateApp(app *appModel.App, _ int) {
	// app.Auth = authModel.Merge(m.Auth, app.Auth)
	// app.Variables.FillMissing(m.Variables)
	// app.InheritDown()
}
