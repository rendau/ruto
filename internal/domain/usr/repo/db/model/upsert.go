package model

import domainModel "github.com/rendau/ruto/internal/domain/usr/model"

type Upsert struct {
	PKId int64

	Name     *string
	Username *string
	Password *string
}

func (m *Upsert) CreateColumnMap() map[string]any {
	result := map[string]any{}

	if m.Name != nil {
		result["name"] = *m.Name
	}
	if m.Username != nil {
		result["username"] = *m.Username
	}
	if m.Password != nil {
		result["password"] = *m.Password
	}

	return result
}

func (m *Upsert) ReturningColumnMap() map[string]any {
	return map[string]any{
		"id": &m.PKId,
	}
}

func (m *Upsert) UpdateColumnMap() map[string]any {
	return m.CreateColumnMap()
}

func (m *Upsert) PKColumnMap() map[string]any {
	return map[string]any{"id": m.PKId}
}

func DecodeUpsert(v *domainModel.Usr) *Upsert {
	return &Upsert{
		Name:     &v.Name,
		Username: &v.Username,
		Password: &v.Password,
	}
}
