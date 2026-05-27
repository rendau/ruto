package model

import domainModel "github.com/rendau/ruto/internal/domain/usr/model"

type Select struct {
	PKId int64

	Active   bool
	IsAdmin  bool
	Name     string
	Username string
	Password string
}

func (m *Select) ListColumnMap() map[string]any {
	return map[string]any{
		"id":       &m.PKId,
		"active":   &m.Active,
		"is_admin": &m.IsAdmin,
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
		Active:   v.Active,
		IsAdmin:  v.IsAdmin,
		Name:     v.Name,
		Username: v.Username,
		Password: v.Password,
	}
}

type GetByUsername struct {
	Select
}

func (m *GetByUsername) PKColumnMap() map[string]any {
	return map[string]any{"username": m.Username}
}
