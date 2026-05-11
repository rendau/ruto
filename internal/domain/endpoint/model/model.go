package model

import (
	commonModel "github.com/rendau/ruto/internal/domain/common/model"
)

type Endpoint struct {
	Id            string        `json:"id"`
	AppId         string        `json:"app_id"`
	Active        bool          `json:"active"`
	Method        string        `json:"method"`
	Path          string        `json:"path"`
	Backend       Backend       `json:"backend"`
	JwtValidation JwtValidation `json:"jwt_validation"`
	IpValidation  IpValidation  `json:"ip_validation"`
}

type Backend struct {
	CustomPath string `json:"custom_path"`
}

type JwtValidation struct {
	Enabled bool     `json:"enabled"`
	Roles   []string `json:"roles"`
}

type IpValidation struct {
	AllowedIps []string `json:"allowed_ips"`
}

type ListReq struct {
	commonModel.ListParams

	AppId  *string
	Active *bool
}
