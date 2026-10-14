package model

import (
	domainModel "github.com/rendau/ruto/internal/domain/changerequest/model"
)

type Create struct {
	PKId int64

	Title       string
	Description string
	Publish     bool
	Plan        domainModel.Plan
	AuthorUsrId int64
	ApiKeyId    string
	ApiKeyName  string
}

func (m *Create) CreateColumnMap() map[string]any {
	return map[string]any{
		"status":        domainModel.StatusPending,
		"title":         m.Title,
		"description":   m.Description,
		"publish":       m.Publish,
		"plan":          m.Plan,
		"author_usr_id": m.AuthorUsrId,
		"api_key_id":    m.ApiKeyId,
		"api_key_name":  m.ApiKeyName,
	}
}

func (m *Create) ReturningColumnMap() map[string]any {
	return map[string]any{"id": &m.PKId}
}

func DecodeCreate(v *domainModel.ChangeRequest) *Create {
	return &Create{
		Title:       v.Title,
		Description: v.Description,
		Publish:     v.Publish,
		Plan:        v.Plan,
		AuthorUsrId: v.AuthorUsrId,
		ApiKeyId:    v.ApiKeyId,
		ApiKeyName:  v.ApiKeyName,
	}
}

type Update struct {
	PKId int64

	Edit *domainModel.Edit
}

func (m *Update) UpdateColumnMap() map[string]any {
	result := map[string]any{}
	e := m.Edit

	if e.Status != nil {
		result["status"] = *e.Status
	}
	if e.DecidedAt != nil {
		result["decided_at"] = *e.DecidedAt
	}
	if e.DecidedUsrId != nil {
		result["decided_usr_id"] = *e.DecidedUsrId
	}
	if e.Comment != nil {
		result["comment"] = *e.Comment
	}
	if e.Error != nil {
		result["error"] = *e.Error
	}
	if e.Result != nil {
		result["result"] = *e.Result
	}
	if e.PublishedAt != nil {
		result["published_at"] = *e.PublishedAt
	}
	if e.PublishError != nil {
		result["publish_error"] = *e.PublishError
	}

	return result
}

func (m *Update) PKColumnMap() map[string]any {
	return map[string]any{"id": m.PKId}
}
