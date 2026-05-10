package model

import (
	"github.com/rendau/ruto/internal/domain/root/model"
)

type Select struct {
	PKId string

	Data model.Root
}

func (m *Select) ListColumnMap() map[string]any {
	return map[string]any{
		"data": &m.Data,
	}
}

func (m *Select) PKColumnMap() map[string]any {
	return map[string]any{"id": m.PKId}
}

func (m *Select) DefaultSortColumns() []string {
	return []string{
		"id",
	}
}

func EncodeSelect(v *Select, _ int) *model.Root {
	return &v.Data
}
