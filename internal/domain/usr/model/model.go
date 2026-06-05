package model

import (
	"strings"

	commonModel "github.com/rendau/ruto/internal/domain/common/model"
)

type Usr struct {
	Id       int64
	Active   bool
	IsAdmin  bool
	AllApps  bool
	AppIds   []string
	Name     string
	Username string
	Password string
}

type ListReq struct {
	commonModel.ListParams

	Search *string
}

type Edit struct {
	Active       *bool
	IsAdmin      *bool
	AllApps      *bool
	UpdateAppIds bool
	AppIds       []string
	Name         *string
	Username     *string
	Password     *string
}

func (m *Usr) Normalize() error {
	m.Name = strings.TrimSpace(m.Name)
	m.Username = strings.TrimSpace(m.Username)
	m.Password = strings.TrimSpace(m.Password)
	return nil
}
