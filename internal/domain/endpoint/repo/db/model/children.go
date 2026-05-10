package model

import domModel "github.com/rendau/ruto/internal/domain/endpoint/model"

type Data struct {
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

func EncodeData(v *Data) *domModel.Data {
	if v == nil {
		return nil
	}
	return &domModel.Data{
		Backend:       EncodeBackend(v.Backend),
		JwtValidation: EncodeJwtValidation(v.JwtValidation),
		IpValidation:  EncodeIpValidation(v.IpValidation),
	}
}

func DecodeData(v *domModel.Data) *Data {
	if v == nil {
		return nil
	}
	return &Data{
		Backend:       DecodeBackend(v.Backend),
		JwtValidation: DecodeJwtValidation(v.JwtValidation),
		IpValidation:  DecodeIpValidation(v.IpValidation),
	}
}

func EncodeBackend(v Backend) domModel.Backend {
	return domModel.Backend{
		CustomPath: v.CustomPath,
	}
}

func DecodeBackend(v domModel.Backend) Backend {
	return Backend{
		CustomPath: v.CustomPath,
	}
}

func EncodeJwtValidation(v JwtValidation) domModel.JwtValidation {
	return domModel.JwtValidation{
		Enabled: v.Enabled,
		Roles:   v.Roles,
	}
}

func DecodeJwtValidation(v domModel.JwtValidation) JwtValidation {
	return JwtValidation{
		Enabled: v.Enabled,
		Roles:   v.Roles,
	}
}

func EncodeIpValidation(v IpValidation) domModel.IpValidation {
	return domModel.IpValidation{
		AllowedIps: v.AllowedIps,
	}
}

func DecodeIpValidation(v domModel.IpValidation) IpValidation {
	return IpValidation{
		AllowedIps: v.AllowedIps,
	}
}
