package model

import (
	"github.com/rendau/ruto/internal/domain/root/model"
)

type Upsert struct {
	PKId string

	Data *model.Root
}

func (m *Upsert) CreateColumnMap() map[string]any {
	result := map[string]any{
		"id": m.PKId,
	}

	if m.Data != nil {
		result["data"] = *m.Data
	}

	return result
}

func (m *Upsert) ReturningColumnMap() map[string]any {
	return map[string]any{}
}

func (m *Upsert) UpdateColumnMap() map[string]any {
	result := m.CreateColumnMap()
	delete(result, "id")
	return result
}

func (m *Upsert) PKColumnMap() map[string]any {
	return map[string]any{"id": m.PKId}
}

func DecodeUpsert(v *model.Root) *Upsert {
	return &Upsert{Data: v}
}
