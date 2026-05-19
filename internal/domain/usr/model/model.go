package model

import (
	"strings"

	commonModel "github.com/rendau/ruto/internal/domain/common/model"
)

type Usr struct {
	Id       int64
	Name     string
	Username string
	Password string
}

type ListReq struct {
	commonModel.ListParams

	Username *string
}

func (m *Usr) Normalize() error {
	m.Name = strings.TrimSpace(m.Name)
	m.Username = strings.TrimSpace(m.Username)
	m.Password = strings.TrimSpace(m.Password)
	return nil
}
