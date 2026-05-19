package model

import domainModel "github.com/rendau/ruto/internal/domain/usr/model"

type Select struct {
	PKId int64

	Name     string
	Username string
	Password string
}

func (m *Select) ListColumnMap() map[string]any {
	return map[string]any{
		"id":       &m.PKId,
		"name":     &m.Name,
		"username": &m.Username,
		"password": &m.Password,
	}
}

func (m *Select) PKColumnMap() map[string]any {
	return map[string]any{"id": m.PKId}
}

func (m *Select) DefaultSortColumns() []string {
	return []string{
		"username", "id",
	}
}

func EncodeSelect(v *Select, _ int) *domainModel.Usr {
	if v == nil {
		return nil
	}
	return &domainModel.Usr{
		Id:       v.PKId,
		Name:     v.Name,
		Username: v.Username,
		Password: v.Password,
	}
}

type GetByUsernameAndPassword struct {
	Select
}

func (m *GetByUsernameAndPassword) PKColumnMap() map[string]any {
	return map[string]any{"username": m.Username, "password": m.Password}
}
