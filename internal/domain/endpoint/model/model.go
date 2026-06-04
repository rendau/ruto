package model

import (
	authModel "github.com/rendau/ruto/internal/domain/auth/model"
	commonModel "github.com/rendau/ruto/internal/domain/common/model"
	varsModel "github.com/rendau/ruto/internal/domain/vars/model"
)

type Type string

const (
	TypeHTTP Type = "http"
	TypeGRPC Type = "grpc"
)

type Endpoint struct {
	Id        string         `json:"id"`
	AppId     string         `json:"app_id"`
	Active    bool           `json:"active"`
	Type      Type           `json:"type"`
	Http      Http           `json:"http"`
	Grpc      Grpc           `json:"grpc"`
	Backend   Backend        `json:"backend"`
	Auth      authModel.Auth `json:"auth"`
	Variables varsModel.Vars `json:"variables"`
}

type Http struct {
	Method string `json:"method"`
	Path   string `json:"path"`
}

type Grpc struct {
	Service string `json:"service"`
	Method  string `json:"method"`
	Path    string `json:"path"`
}

type Backend struct {
	CustomPath  string         `json:"custom_path"`
	Headers     varsModel.Vars `json:"headers"`
	QueryParams varsModel.Vars `json:"query_params"`
}

type ListReq struct {
	commonModel.ListParams

	AppId  *string
	Active *bool
}
