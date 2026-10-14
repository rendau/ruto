package model

import (
	"time"

	domainModel "github.com/rendau/ruto/internal/domain/apikey/model"
)

type Select struct {
	PKId string

	UsrId      int64
	Name       string
	KeyHash    string
	KeyPrefix  string
	CreatedAt  time.Time
	LastUsedAt *time.Time
}

func (m *Select) ListColumnMap() map[string]any {
	return map[string]any{
		"id":           &m.PKId,
		"usr_id":       &m.UsrId,
		"name":         &m.Name,
		"key_hash":     &m.KeyHash,
		"key_prefix":   &m.KeyPrefix,
		"created_at":   &m.CreatedAt,
		"last_used_at": &m.LastUsedAt,
	}
}

func (m *Select) PKColumnMap() map[string]any {
	return map[string]any{"id": m.PKId}
}

func (m *Select) DefaultSortColumns() []string {
	return []string{"created_at", "id"}
}

func EncodeSelect(v *Select, _ int) *domainModel.ApiKey {
	if v == nil {
		return nil
	}
	return &domainModel.ApiKey{
		Id:         v.PKId,
		UsrId:      v.UsrId,
		Name:       v.Name,
		KeyHash:    v.KeyHash,
		KeyPrefix:  v.KeyPrefix,
		CreatedAt:  v.CreatedAt,
		LastUsedAt: v.LastUsedAt,
	}
}
