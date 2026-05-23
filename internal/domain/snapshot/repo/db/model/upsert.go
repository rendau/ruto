package model

import (
	"github.com/rendau/ruto/internal/domain/snapshot/model"
)

type Upsert struct {
	PKId string

	Hash string
	Data []byte
}

func (m *Upsert) CreateColumnMap() map[string]any {
	return map[string]any{
		"id":   m.PKId,
		"hash": m.Hash,
		"data": m.Data,
	}
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

func DecodeUpsert(v *model.Snapshot) *Upsert {
	return &Upsert{
		Hash: v.Hash,
		Data: v.Data,
	}
}
