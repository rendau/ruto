package model

import (
	"github.com/samber/lo"

	appModel "github.com/rendau/ruto/internal/domain/app/model"
	varsModel "github.com/rendau/ruto/internal/domain/vars/model"
)

// Redacted returns a copy of root with all secret material masked (auth
// secrets, variable values, and any nested apps). It is used when root is
// returned to or inherited for a non-admin user. The receiver and its nested
// maps/slices are left untouched.
func (m *Root) Redacted() *Root {
	result := *m

	result.Auth = m.Auth.Redacted()
	result.Variables = varsModel.RedactedValues(m.Variables)

	if len(m.Apps) > 0 {
		result.Apps = lo.Map(m.Apps, func(app *appModel.App, _ int) *appModel.App {
			return app.Redacted()
		})
	}

	return &result
}
