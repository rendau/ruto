package model

import "github.com/rendau/ruto/internal/domain/app/model"

type Select struct {
	PKId string

	Active bool
	Data   model.App
}

func (m *Select) ListColumnMap() map[string]any {
	return map[string]any{
		"id":     &m.PKId,
		"active": &m.Active,
		"data":   &m.Data,
	}
}

func (m *Select) PKColumnMap() map[string]any {
	return map[string]any{"id": m.PKId}
}

func (m *Select) DefaultSortColumns() []string {
	return []string{
		"data ->> 'name'", "id",
	}
}

func EncodeSelect(v *Select, _ int) *model.App {
	if v == nil {
		return nil
	}
	v.Data.Id = v.PKId
	v.Data.Active = v.Active
	return &v.Data
}
