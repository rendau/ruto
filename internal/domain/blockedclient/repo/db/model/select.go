package model

import (
	"time"

	domainModel "github.com/rendau/ruto/internal/domain/blockedclient/model"
)

type Select struct {
	PKId string

	Ip            string
	Reason        string
	CreatedBy     string
	Source        string
	ApiKeyId      string
	CreatedAt     time.Time
	UpdatedAt     time.Time
	ExpiresAt     *time.Time
	RemovedAt     *time.Time
	RemovedBy     string
	RemoveReason  string
	RemovedSource string
}

func (m *Select) ListColumnMap() map[string]any {
	return map[string]any{
		"id":             &m.PKId,
		"ip":             &m.Ip,
		"reason":         &m.Reason,
		"created_by":     &m.CreatedBy,
		"source":         &m.Source,
		"api_key_id":     &m.ApiKeyId,
		"created_at":     &m.CreatedAt,
		"updated_at":     &m.UpdatedAt,
		"expires_at":     &m.ExpiresAt,
		"removed_at":     &m.RemovedAt,
		"removed_by":     &m.RemovedBy,
		"remove_reason":  &m.RemoveReason,
		"removed_source": &m.RemovedSource,
	}
}

func (m *Select) PKColumnMap() map[string]any {
	return map[string]any{"id": m.PKId}
}

func (m *Select) DefaultSortColumns() []string {
	return []string{"created_at DESC", "id"}
}

func EncodeSelect(v *Select, _ int) *domainModel.BlockedClient {
	if v == nil {
		return nil
	}
	return &domainModel.BlockedClient{
		Id:            v.PKId,
		Ip:            v.Ip,
		Reason:        v.Reason,
		CreatedBy:     v.CreatedBy,
		Source:        v.Source,
		ApiKeyId:      v.ApiKeyId,
		CreatedAt:     v.CreatedAt,
		UpdatedAt:     v.UpdatedAt,
		ExpiresAt:     v.ExpiresAt,
		RemovedAt:     v.RemovedAt,
		RemovedBy:     v.RemovedBy,
		RemoveReason:  v.RemoveReason,
		RemovedSource: v.RemovedSource,
	}
}
