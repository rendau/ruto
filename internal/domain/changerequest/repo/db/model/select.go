package model

import (
	"time"

	domainModel "github.com/rendau/ruto/internal/domain/changerequest/model"
)

type Select struct {
	PKId int64

	Status       string
	Title        string
	Description  string
	Publish      bool
	Plan         domainModel.Plan
	CreatedAt    time.Time
	AuthorUsrId  int64
	ApiKeyId     string
	ApiKeyName   string
	DecidedAt    *time.Time
	DecidedUsrId int64
	Comment      string
	Error        string
	Result       domainModel.Result
	PublishedAt  *time.Time
	PublishError string
}

func (m *Select) ListColumnMap() map[string]any {
	return map[string]any{
		"id":             &m.PKId,
		"status":         &m.Status,
		"title":          &m.Title,
		"description":    &m.Description,
		"publish":        &m.Publish,
		"plan":           &m.Plan,
		"created_at":     &m.CreatedAt,
		"author_usr_id":  &m.AuthorUsrId,
		"api_key_id":     &m.ApiKeyId,
		"api_key_name":   &m.ApiKeyName,
		"decided_at":     &m.DecidedAt,
		"decided_usr_id": &m.DecidedUsrId,
		"comment":        &m.Comment,
		"error":          &m.Error,
		"result":         &m.Result,
		"published_at":   &m.PublishedAt,
		"publish_error":  &m.PublishError,
	}
}

func (m *Select) PKColumnMap() map[string]any {
	return map[string]any{"id": m.PKId}
}

func (m *Select) DefaultSortColumns() []string {
	return []string{"id desc"}
}

func EncodeSelect(v *Select, _ int) *domainModel.ChangeRequest {
	if v == nil {
		return nil
	}
	return &domainModel.ChangeRequest{
		Id:           v.PKId,
		Status:       v.Status,
		Title:        v.Title,
		Description:  v.Description,
		Publish:      v.Publish,
		Plan:         v.Plan,
		CreatedAt:    v.CreatedAt,
		AuthorUsrId:  v.AuthorUsrId,
		ApiKeyId:     v.ApiKeyId,
		ApiKeyName:   v.ApiKeyName,
		DecidedAt:    v.DecidedAt,
		DecidedUsrId: v.DecidedUsrId,
		Comment:      v.Comment,
		Error:        v.Error,
		Result:       v.Result,
		PublishedAt:  v.PublishedAt,
		PublishError: v.PublishError,
	}
}
