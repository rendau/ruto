package model

import (
	"time"

	domainModel "github.com/rendau/ruto/internal/domain/blockedclient/model"
)

type Create struct {
	PKId string

	Ip        string
	Reason    string
	CreatedBy string
	Source    string
	ApiKeyId  string
	CreatedAt time.Time
	ExpiresAt *time.Time
}

func (m *Create) CreateColumnMap() map[string]any {
	return map[string]any{
		"ip":         m.Ip,
		"reason":     m.Reason,
		"created_by": m.CreatedBy,
		"source":     m.Source,
		"api_key_id": m.ApiKeyId,
		"created_at": m.CreatedAt,
		"updated_at": m.CreatedAt,
		"expires_at": m.ExpiresAt,
	}
}

func (m *Create) ReturningColumnMap() map[string]any {
	return map[string]any{"id": &m.PKId}
}

func DecodeCreate(v *domainModel.BlockedClient) *Create {
	return &Create{
		Ip:        v.Ip,
		Reason:    v.Reason,
		CreatedBy: v.CreatedBy,
		Source:    v.Source,
		ApiKeyId:  v.ApiKeyId,
		CreatedAt: v.CreatedAt,
		ExpiresAt: v.ExpiresAt,
	}
}

type Update struct {
	PKId string

	Edit *domainModel.Edit
}

func (m *Update) UpdateColumnMap() map[string]any {
	result := map[string]any{}
	e := m.Edit

	if e.Reason != nil {
		result["reason"] = *e.Reason
	}
	if e.UpdatedAt != nil {
		result["updated_at"] = *e.UpdatedAt
	}
	if e.Source != nil {
		result["source"] = *e.Source
	}
	if e.SetExpiresAt {
		result["expires_at"] = e.ExpiresAt
	}
	if e.RemovedAt != nil {
		result["removed_at"] = *e.RemovedAt
	}
	if e.RemovedBy != nil {
		result["removed_by"] = *e.RemovedBy
	}
	if e.RemoveReason != nil {
		result["remove_reason"] = *e.RemoveReason
	}
	if e.RemovedSource != nil {
		result["removed_source"] = *e.RemovedSource
	}

	return result
}

func (m *Update) PKColumnMap() map[string]any {
	return map[string]any{"id": m.PKId}
}
