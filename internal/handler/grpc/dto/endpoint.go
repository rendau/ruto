package dto

import (
	"github.com/samber/lo"

	"github.com/rendau/ruto/pkg/proto/ruto_v1"

	"github.com/rendau/ruto/internal/domain/endpoint/model"
)

func EncodeEndpointMain(v *model.Endpoint, _ int) *ruto_v1.EndpointMain {
	return &ruto_v1.EndpointMain{
		Id:           v.Id,
		AppId:        v.AppId,
		Active:       v.Active,
		Method:       v.Method,
		Path:         v.Path,
		Backend:      EncodeEndpointBackend(v.Backend),
		Auth:         EncodeEndpointAuth(v.Auth),
		IpValidation: EncodeEndpointIpValidation(v.IpValidation),
	}
}

func DecodeEndpointMain(v *ruto_v1.EndpointMain) *model.Endpoint {
	return &model.Endpoint{
		Id:           v.Id,
		AppId:        v.AppId,
		Active:       v.Active,
		Method:       v.Method,
		Path:         v.Path,
		Backend:      DecodeEndpointBackend(v.Backend),
		Auth:         DecodeEndpointAuth(v.Auth),
		IpValidation: DecodeEndpointIpValidation(v.IpValidation),
	}
}

func DecodeEndpointListReq(v *ruto_v1.EndpointListReq) *model.ListReq {
	return &model.ListReq{
		ListParams: DecodeListParams(v.ListParams),
		AppId:      v.AppId,
		Active:     v.Active,
	}
}

func EncodeEndpointBackend(x model.Backend) *ruto_v1.EndpointBackend {
	return &ruto_v1.EndpointBackend{
		CustomPath: x.CustomPath,
	}
}

func DecodeEndpointBackend(x *ruto_v1.EndpointBackend) model.Backend {
	if x == nil {
		return model.Backend{}
	}
	return model.Backend{
		CustomPath: x.CustomPath,
	}
}

func EncodeEndpointAuth(x model.Auth) *ruto_v1.EndpointAuth {
	return &ruto_v1.EndpointAuth{
		Enabled: x.Enabled,
		Methods: lo.Map(x.Methods, EncodeEndpointAuthMethod),
	}
}

func DecodeEndpointAuth(x *ruto_v1.EndpointAuth) model.Auth {
	if x == nil {
		return model.Auth{}
	}
	return model.Auth{
		Enabled: x.Enabled,
		Methods: lo.FilterMap(x.Methods, DecodeEndpointAuthMethod),
	}
}

func EncodeEndpointAuthMethod(x model.AuthMethod, _ int) *ruto_v1.EndpointAuthMethod {
	result := &ruto_v1.EndpointAuthMethod{}
	if x.Basic != nil {
		result.Method = &ruto_v1.EndpointAuthMethod_Basic{
			Basic: EncodeEndpointAuthMethodBasic(*x.Basic),
		}
	} else if x.APIKey != nil {
		result.Method = &ruto_v1.EndpointAuthMethod_ApiKey{
			ApiKey: EncodeEndpointAuthMethodAPIKey(*x.APIKey),
		}
	} else if x.JWT != nil {
		result.Method = &ruto_v1.EndpointAuthMethod_Jwt{
			Jwt: EncodeEndpointAuthMethodJWT(*x.JWT),
		}
	}
	return result
}

func DecodeEndpointAuthMethod(x *ruto_v1.EndpointAuthMethod, _ int) (model.AuthMethod, bool) {
	if x == nil {
		return model.AuthMethod{}, false
	}

	switch v := x.Method.(type) {
	case *ruto_v1.EndpointAuthMethod_Basic:
		return model.AuthMethod{
			Basic: new(DecodeEndpointAuthMethodBasic(v.Basic)),
		}, true
	case *ruto_v1.EndpointAuthMethod_ApiKey:
		return model.AuthMethod{
			APIKey: new(DecodeEndpointAuthMethodAPIKey(v.ApiKey)),
		}, true
	case *ruto_v1.EndpointAuthMethod_Jwt:
		return model.AuthMethod{
			JWT: new(DecodeEndpointAuthMethodJWT(v.Jwt)),
		}, true
	default:
		return model.AuthMethod{}, false
	}
}

func EncodeEndpointAuthMethodBasic(x model.AuthMethodBasic) *ruto_v1.EndpointAuthMethodBasic {
	return &ruto_v1.EndpointAuthMethodBasic{
		Users: lo.Map(x.Users, EncodeEndpointAuthMethodBasicUser),
	}
}

func DecodeEndpointAuthMethodBasic(x *ruto_v1.EndpointAuthMethodBasic) model.AuthMethodBasic {
	if x == nil {
		return model.AuthMethodBasic{}
	}
	return model.AuthMethodBasic{
		Users: lo.FilterMap(x.Users, DecodeEndpointAuthMethodBasicUser),
	}
}

func EncodeEndpointAuthMethodBasicUser(x model.AuthMethodBasicUser, _ int) *ruto_v1.EndpointAuthMethodBasicUser {
	return &ruto_v1.EndpointAuthMethodBasicUser{
		Username: x.Username,
		Password: x.Password,
	}
}

func DecodeEndpointAuthMethodBasicUser(x *ruto_v1.EndpointAuthMethodBasicUser, _ int) (model.AuthMethodBasicUser, bool) {
	if x == nil {
		return model.AuthMethodBasicUser{}, false
	}
	return model.AuthMethodBasicUser{
		Username: x.Username,
		Password: x.Password,
	}, true
}

func EncodeEndpointAuthMethodAPIKey(x model.AuthMethodAPIKey) *ruto_v1.EndpointAuthMethodAPIKey {
	return &ruto_v1.EndpointAuthMethodAPIKey{
		Header: x.Header,
		Keys:   x.Keys,
	}
}

func DecodeEndpointAuthMethodAPIKey(x *ruto_v1.EndpointAuthMethodAPIKey) model.AuthMethodAPIKey {
	if x == nil {
		return model.AuthMethodAPIKey{}
	}
	return model.AuthMethodAPIKey{
		Header: x.Header,
		Keys:   x.Keys,
	}
}

func EncodeEndpointAuthMethodJWT(x model.AuthMethodJWT) *ruto_v1.EndpointAuthMethodJWT {
	return &ruto_v1.EndpointAuthMethodJWT{
		Kids:  x.Kids,
		Roles: x.Roles,
	}
}

func DecodeEndpointAuthMethodJWT(x *ruto_v1.EndpointAuthMethodJWT) model.AuthMethodJWT {
	if x == nil {
		return model.AuthMethodJWT{}
	}
	return model.AuthMethodJWT{
		Kids:  x.Kids,
		Roles: x.Roles,
	}
}

func EncodeEndpointIpValidation(x model.IpValidation) *ruto_v1.EndpointIpValidation {
	return &ruto_v1.EndpointIpValidation{
		AllowedIps: x.AllowedIps,
	}
}

func DecodeEndpointIpValidation(x *ruto_v1.EndpointIpValidation) model.IpValidation {
	if x == nil {
		return model.IpValidation{}
	}
	return model.IpValidation{
		AllowedIps: x.AllowedIps,
	}
}
