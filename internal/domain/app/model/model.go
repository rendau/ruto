package model

import (
	commonModel "github.com/rendau/ruto/internal/domain/common/model"
)

type App struct {
	Id         string     `json:"id"`
	Active     bool       `json:"active"`
	PathPrefix string     `json:"path_prefix"`
	Name       string     `json:"name"`
	Backend    AppBackend `json:"backend"`
}

type AppBackend struct {
	Url string `json:"url"`
}

type ListReq struct {
	commonModel.ListParams

	Active *bool
}
