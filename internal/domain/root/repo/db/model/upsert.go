package model

import (
	"github.com/samber/lo"

	domModel "github.com/rendau/ruto/internal/domain/root/model"
)

type Upsert struct {
	PKId string

	PublicBaseUrl *string
	Cors          *Cors
	Jwt           *[]*Jwt
}

func (m *Upsert) CreateColumnMap() map[string]any {
	result := map[string]any{
		"id": m.PKId,
	}

	if m.PublicBaseUrl != nil {
		result["public_base_url"] = *m.PublicBaseUrl
	}
	if m.Cors != nil {
		result["cors"] = *m.Cors
	}
	if m.Jwt != nil {
		result["jwt"] = *m.Jwt
	}

	return result
}

func (m *Upsert) ReturningColumnMap() map[string]any { return map[string]any{} }

func (m *Upsert) UpdateColumnMap() map[string]any {
	result := m.CreateColumnMap()
	delete(result, "id")
	return result
}

func (m *Upsert) PKColumnMap() map[string]any {
	return map[string]any{"id": m.PKId}
}

func DecodeUpsert(v *domModel.Edit) *Upsert {
	result := &Upsert{
		PublicBaseUrl: v.PublicBaseUrl,
		Cors:          DecodeCors(v.Cors),
	}

	if v.Jwt != nil {
		result.Jwt = new(lo.Map(*v.Jwt, DecodeJwt))
	}

	return result
}
