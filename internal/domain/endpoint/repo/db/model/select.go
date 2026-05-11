package model

import "github.com/rendau/ruto/internal/domain/endpoint/model"

type Select struct {
	PKId string

	AppId  string
	Active bool
	Data   model.Endpoint
}

func (m *Select) ListColumnMap() map[string]any {
	return map[string]any{
		"id":     &m.PKId,
		"app_id": &m.AppId,
		"active": &m.Active,
		"data":   &m.Data,
	}
}

func (m *Select) PKColumnMap() map[string]any {
	return map[string]any{"id": m.PKId}
}

func (m *Select) DefaultSortColumns() []string {
	return []string{
		"active desc",
		"app_id",
		"data ->> 'path'",
		"data ->> 'method'",
		"id",
	}
}

func EncodeSelect(v *Select, _ int) *model.Endpoint {
	if v == nil {
		return nil
	}
	v.Data.Id = v.PKId
	v.Data.AppId = v.AppId
	v.Data.Active = v.Active
	return &v.Data
}
