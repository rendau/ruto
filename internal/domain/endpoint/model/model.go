package model

import (
	commonModel "github.com/rendau/ruto/internal/domain/common/model"
)

type Main struct {
	Id     string
	AppId  string
	Active bool
	Method string
	Path   string
	Data   *Data
}

type ListReq struct {
	commonModel.ListParams

	AppId  *string
	Active *bool
}

type Edit struct {
	AppId  *string
	Active *bool
	Method *string
	Path   *string
	Data   *Data
}

// child models

type Data struct {
	Backend       Backend
	JwtValidation JwtValidation
	IpValidation  IpValidation
}

type Backend struct {
	CustomPath string
}

type JwtValidation struct {
	Enabled bool
	Roles   []string
}

type IpValidation struct {
	AllowedIps []string
}
