package model

import (
	commonModel "github.com/rendau/ruto/internal/domain/common/model"
)

type Main struct {
	Id         string
	Active     bool
	PathPrefix string
	Name       string
	Backend    *Backend
}

type ListReq struct {
	commonModel.ListParams

	Active *bool
}

type Edit struct {
	Active     *bool
	PathPrefix *string
	Name       *string
	Backend    *Backend
}

// child models

type Backend struct {
	Url string
}
