package model

import (
	"github.com/rendau/ruto/internal/domain/app/model"
)

type Upsert struct {
	PKId string

	Id     *string
	Active *bool
	Data   *model.App
}

func (m *Upsert) CreateColumnMap() map[string]any {
	result := map[string]any{}

	if m.Id != nil {
		result["id"] = *m.Id
	}
	if m.Active != nil {
		result["active"] = *m.Active
	}
	if m.Data != nil {
		result["data"] = *m.Data
	}

	return result
}

func (m *Upsert) ReturningColumnMap() map[string]any {
	return map[string]any{
		"id": &m.PKId,
	}
}

func (m *Upsert) UpdateColumnMap() map[string]any {
	result := m.CreateColumnMap()
	for k, _ := range m.PKColumnMap() {
		delete(result, k)
	}
	return result
}

func (m *Upsert) PKColumnMap() map[string]any {
	return map[string]any{"id": m.PKId}
}

func DecodeUpsert(v *model.App) *Upsert {
	result := &Upsert{
		Active: &v.Active,
		Data:   v,
	}
	if v.Id != "" {
		result.Id = &v.Id
	}
	return result
}
