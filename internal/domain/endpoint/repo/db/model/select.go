package model

import domModel "github.com/rendau/ruto/internal/domain/endpoint/model"

type Select struct {
	PKId string

	AppId  string
	Active bool
	Method string
	Path   string
	Data   *Data
}

func (m *Select) ListColumnMap() map[string]any {
	return map[string]any{
		"id":     &m.PKId,
		"app_id": &m.AppId,
		"active": &m.Active,
		"method": &m.Method,
		"path":   &m.Path,
		"data":   &m.Data,
	}
}

func (m *Select) PKColumnMap() map[string]any { return map[string]any{"id": m.PKId} }

func (m *Select) DefaultSortColumns() []string {
	return []string{
		"active desc",
		"app_id",
		"path",
		"method",
		"id",
	}
}

func EncodeSelect(v *Select, _ int) *domModel.Main {
	if v == nil {
		return nil
	}
	return &domModel.Main{
		Id:     v.PKId,
		AppId:  v.AppId,
		Active: v.Active,
		Method: v.Method,
		Path:   v.Path,
		Data:   EncodeData(v.Data),
	}
}
