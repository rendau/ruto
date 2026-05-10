package model

import domModel "github.com/rendau/ruto/internal/domain/app/model"

type Upsert struct {
	PKId string

	Active     *bool
	PathPrefix *string
	Name       *string
	Backend    *Backend
}

func (m *Upsert) CreateColumnMap() map[string]any {
	result := map[string]any{}

	if m.Active != nil {
		result["active"] = *m.Active
	}
	if m.PathPrefix != nil {
		result["path_prefix"] = *m.PathPrefix
	}
	if m.Name != nil {
		result["name"] = *m.Name
	}
	if m.Backend != nil {
		result["backend"] = *m.Backend
	}

	return result
}

func (m *Upsert) ReturningColumnMap() map[string]any {
	return map[string]any{
		"id": &m.PKId,
	}
}

func (m *Upsert) UpdateColumnMap() map[string]any { return m.CreateColumnMap() }

func (m *Upsert) PKColumnMap() map[string]any { return map[string]any{"id": m.PKId} }

func DecodeUpsert(v *domModel.Edit) *Upsert {
	if v == nil {
		return nil
	}
	return &Upsert{
		Active:     v.Active,
		PathPrefix: v.PathPrefix,
		Name:       v.Name,
		Backend:    DecodeBackend(v.Backend),
	}
}
