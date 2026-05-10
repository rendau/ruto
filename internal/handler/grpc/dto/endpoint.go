package dto

import (
	"github.com/rendau/ruto/pkg/proto/ruto_v1"

	domModel "github.com/rendau/ruto/internal/domain/endpoint/model"
)

func EncodeEndpointMain(v *domModel.Main, _ int) *ruto_v1.EndpointMain {
	if v == nil {
		return nil
	}

	return &ruto_v1.EndpointMain{
		Id:     v.Id,
		AppId:  v.AppId,
		Active: v.Active,
		Method: v.Method,
		Path:   v.Path,
		Data:   EncodeEndpointData(v.Data),
	}
}

func DecodeEndpointListReq(v *ruto_v1.EndpointListReq) *domModel.ListReq {
	if v == nil {
		return nil
	}
	return &domModel.ListReq{
		ListParams: DecodeListParams(v.ListParams),
		AppId:      v.AppId,
		Active:     v.Active,
	}
}

func DecodeEndpointCreateReq(v *ruto_v1.EndpointCreateReq) *domModel.Edit {
	if v == nil {
		return nil
	}

	return &domModel.Edit{
		AppId:  v.AppId,
		Active: v.Active,
		Method: v.Method,
		Path:   v.Path,
		Data:   DecodeEndpointData(v.Data),
	}
}

func DecodeEndpointUpdateReq(v *ruto_v1.EndpointUpdateReq) *domModel.Edit {
	if v == nil {
		return nil
	}

	return &domModel.Edit{
		AppId:  v.AppId,
		Active: v.Active,
		Method: v.Method,
		Path:   v.Path,
		Data:   DecodeEndpointData(v.Data),
	}
}

func EncodeEndpointData(x *domModel.Data) *ruto_v1.EndpointData {
	if x == nil {
		return nil
	}
	return &ruto_v1.EndpointData{
		Backend:       EncodeEndpointBackend(x.Backend),
		JwtValidation: EncodeEndpointJwtValidation(x.JwtValidation),
		IpValidation:  EncodeEndpointIpValidation(x.IpValidation),
	}
}

func DecodeEndpointData(x *ruto_v1.EndpointData) *domModel.Data {
	if x == nil {
		return nil
	}
	return &domModel.Data{
		Backend:       DecodeEndpointBackend(x.Backend),
		JwtValidation: DecodeEndpointJwtValidation(x.JwtValidation),
		IpValidation:  DecodeEndpointIpValidation(x.IpValidation),
	}
}

func EncodeEndpointBackend(x domModel.Backend) *ruto_v1.EndpointBackend {
	return &ruto_v1.EndpointBackend{
		CustomPath: x.CustomPath,
	}
}

func DecodeEndpointBackend(x *ruto_v1.EndpointBackend) domModel.Backend {
	if x == nil {
		return domModel.Backend{}
	}
	return domModel.Backend{
		CustomPath: x.CustomPath,
	}
}

func EncodeEndpointJwtValidation(x domModel.JwtValidation) *ruto_v1.EndpointJwtValidation {
	return &ruto_v1.EndpointJwtValidation{
		Enabled: x.Enabled,
		Roles:   x.Roles,
	}
}

func DecodeEndpointJwtValidation(x *ruto_v1.EndpointJwtValidation) domModel.JwtValidation {
	if x == nil {
		return domModel.JwtValidation{}
	}
	return domModel.JwtValidation{
		Enabled: x.Enabled,
		Roles:   x.Roles,
	}
}

func EncodeEndpointIpValidation(x domModel.IpValidation) *ruto_v1.EndpointIpValidation {
	return &ruto_v1.EndpointIpValidation{
		AllowedIps: x.AllowedIps,
	}
}

func DecodeEndpointIpValidation(x *ruto_v1.EndpointIpValidation) domModel.IpValidation {
	if x == nil {
		return domModel.IpValidation{}
	}
	return domModel.IpValidation{
		AllowedIps: x.AllowedIps,
	}
}
