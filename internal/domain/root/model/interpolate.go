package model

func (m *Root) Interpolate() {
	if len(m.Variables) > 0 {
		m.Auth.Interpolate(m.Variables)
	}

	for _, app := range m.Apps {
		app.Interpolate()
	}
}
