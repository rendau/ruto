package model

import domModel "github.com/rendau/ruto/internal/domain/app/model"

type Select struct {
	PKId string

	Active     bool
	PathPrefix string
	Name       string
	Backend    *Backend
}

func (m *Select) ListColumnMap() map[string]any {
	return map[string]any{
		"id":          &m.PKId,
		"active":      &m.Active,
		"path_prefix": &m.PathPrefix,
		"name":        &m.Name,
		"backend":     &m.Backend,
	}
}

func (m *Select) PKColumnMap() map[string]any { return map[string]any{"id": m.PKId} }

func (m *Select) DefaultSortColumns() []string {
	return []string{
		"name", "id",
	}
}

func EncodeSelect(v *Select, _ int) *domModel.Main {
	if v == nil {
		return nil
	}
	return &domModel.Main{
		Id:         v.PKId,
		Active:     v.Active,
		PathPrefix: v.PathPrefix,
		Name:       v.Name,
		Backend:    EncodeBackend(v.Backend),
	}
}
