package model

import (
	domainModel "github.com/rendau/ruto/internal/domain/apikey/model"
)

type Upsert struct {
	PKId string

	UsrId     int64
	Name      string
	KeyHash   string
	KeyPrefix string
}

func (m *Upsert) CreateColumnMap() map[string]any {
	return map[string]any{
		"usr_id":     m.UsrId,
		"name":       m.Name,
		"key_hash":   m.KeyHash,
		"key_prefix": m.KeyPrefix,
	}
}

func (m *Upsert) ReturningColumnMap() map[string]any {
	return map[string]any{"id": &m.PKId}
}

func (m *Upsert) PKColumnMap() map[string]any {
	return map[string]any{"id": m.PKId}
}

func DecodeUpsert(v *domainModel.ApiKey) *Upsert {
	return &Upsert{
		UsrId:     v.UsrId,
		Name:      v.Name,
		KeyHash:   v.KeyHash,
		KeyPrefix: v.KeyPrefix,
	}
}
