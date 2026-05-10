package model

import domModel "github.com/rendau/ruto/internal/domain/endpoint/model"

type Upsert struct {
	PKId string

	AppId  *string
	Active *bool
	Method *string
	Path   *string
	Data   *Data
}

func (m *Upsert) CreateColumnMap() map[string]any {
	result := map[string]any{}

	if m.AppId != nil {
		result["app_id"] = *m.AppId
	}
	if m.Active != nil {
		result["active"] = *m.Active
	}
	if m.Method != nil {
		result["method"] = *m.Method
	}
	if m.Path != nil {
		result["path"] = *m.Path
	}
	if m.Data != nil {
		result["data"] = *m.Data
	}

	return result
}

func (m *Upsert) ReturningColumnMap() map[string]any {
	return map[string]any{
		"id": &m.PKId,
	}
}

func (m *Upsert) UpdateColumnMap() map[string]any { return m.CreateColumnMap() }

func (m *Upsert) PKColumnMap() map[string]any { return map[string]any{"id": m.PKId} }

func DecodeUpsert(v *domModel.Edit) *Upsert {
	if v == nil {
		return nil
	}
	return &Upsert{
		AppId:  v.AppId,
		Active: v.Active,
		Method: v.Method,
		Path:   v.Path,
		Data:   DecodeData(v.Data),
	}
}
