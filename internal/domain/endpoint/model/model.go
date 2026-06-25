package model

import (
	authModel "github.com/rendau/ruto/internal/domain/auth/model"
	commonModel "github.com/rendau/ruto/internal/domain/common/model"
	loggingModel "github.com/rendau/ruto/internal/domain/logging/model"
	varsModel "github.com/rendau/ruto/internal/domain/vars/model"
)

type Type string

const (
	TypeHTTP Type = "http"
	TypeGRPC Type = "grpc"
)

type Endpoint struct {
	Id                 string               `json:"id"`
	AppId              string               `json:"app_id"`
	Active             bool                 `json:"active"`
	ExcludeFromMetrics bool                 `json:"exclude_from_metrics"`
	Type               Type                 `json:"type"`
	Http               Http                 `json:"http"`
	Grpc               Grpc                 `json:"grpc"`
	Backend            Backend              `json:"backend"`
	Auth               authModel.Auth       `json:"auth"`
	Logging            loggingModel.Logging `json:"logging"`
	Variables          varsModel.Vars       `json:"variables"`
	Transform          Transform            `json:"transform"`
}

// Transform holds optional scripts (JavaScript, evaluated by the gateway) that
// reshape a request before it is proxied to the backend.
type Transform struct {
	Request string `json:"request"`
	// MaxWorkers caps concurrent goja runtimes for this script (memory bound).
	// 0 falls back to the Root-level default, then the engine default.
	MaxWorkers int `json:"max_workers"`
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
