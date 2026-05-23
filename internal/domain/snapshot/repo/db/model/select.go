package model

import (
	"github.com/rendau/ruto/internal/domain/snapshot/model"
)

type Select struct {
	PKId string

	Hash string
	Data []byte
}

func (m *Select) ListColumnMap() map[string]any {
	return map[string]any{
		"hash": &m.Hash,
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

func EncodeSelect(v *Select, _ int) *model.Snapshot {
	return &model.Snapshot{
		Hash: v.Hash,
		Data: v.Data,
	}
}
