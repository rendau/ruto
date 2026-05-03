package model

import (
	"github.com/samber/lo"

	domModel "github.com/rendau/ruto/internal/domain/root/model"
)

type Select struct {
	PKId string

	PublicBaseUrl string
	Cors          *Cors
	Jwt           []*Jwt
}

func (m *Select) ListColumnMap() map[string]any {
	return map[string]any{
		"public_base_url": &m.PublicBaseUrl,
		"cors":            &m.Cors,
		"jwt":             &m.Jwt,
	}
}

func (m *Select) PKColumnMap() map[string]any { return map[string]any{"id": m.PKId} }

func (m *Select) DefaultSortColumns() []string {
	return []string{
		"id",
	}
}

func EncodeSelect(v *Select, _ int) *domModel.Main {
	return &domModel.Main{
		PublicBaseUrl: v.PublicBaseUrl,
		Cors:          EncodeCors(v.Cors),
		Jwt:           lo.Map(v.Jwt, EncodeJwt),
	}
}
