package dto

import (
	"github.com/samber/lo"

	"github.com/rendau/ruto/pkg/proto/ruto_v1"

	"github.com/rendau/ruto/internal/domain/endpoint/model"
)

func EncodeEndpointMain(v *model.Endpoint, _ int) *ruto_v1.EndpointMain {
	return &ruto_v1.EndpointMain{
		Id:      v.Id,
		AppId:   v.AppId,
		Active:  v.Active,
		Method:  v.Method,
		Path:    v.Path,
		Backend: EncodeEndpointBackend(v.Backend),
		Auth:    EncodeEndpointAuth(v.Auth),
	}
}

func DecodeEndpointMain(v *ruto_v1.EndpointMain) *model.Endpoint {
	return &model.Endpoint{
		Id:      v.Id,
		AppId:   v.AppId,
		Active:  v.Active,
		Method:  v.Method,
		Path:    v.Path,
		Backend: DecodeEndpointBackend(v.Backend),
		Auth:    DecodeEndpointAuth(v.Auth),
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
	result := &ruto_v1.EndpointAuthMethod{
		Basic:        EncodeEndpointAuthMethodBasicPtr(x.Basic),
		ApiKey:       EncodeEndpointAuthMethodAPIKeyPtr(x.APIKey),
		Jwt:          EncodeEndpointAuthMethodJWTPtr(x.JWT),
		IpValidation: EncodeEndpointAuthMethodIPValidationPtr(x.IPValidation),
	}
	return result
}

func EncodeEndpointAuthMethodBasicPtr(x *model.AuthMethodBasic) *ruto_v1.EndpointAuthMethodBasic {
	if x != nil {
		return EncodeEndpointAuthMethodBasic(*x)
	}
	return nil
}

func EncodeEndpointAuthMethodAPIKeyPtr(x *model.AuthMethodAPIKey) *ruto_v1.EndpointAuthMethodAPIKey {
	if x != nil {
		return EncodeEndpointAuthMethodAPIKey(*x)
	}
	return nil
}

func EncodeEndpointAuthMethodJWTPtr(x *model.AuthMethodJWT) *ruto_v1.EndpointAuthMethodJWT {
	if x != nil {
		return EncodeEndpointAuthMethodJWT(*x)
	}
	return nil
}

func EncodeEndpointAuthMethodIPValidationPtr(x *model.AuthMethodIPValidation) *ruto_v1.EndpointAuthMethodIPValidation {
	if x != nil {
		return EncodeEndpointAuthMethodIPValidation(*x)
	}
	return nil
}

func DecodeEndpointAuthMethod(x *ruto_v1.EndpointAuthMethod, _ int) (model.AuthMethod, bool) {
	if x == nil {
		return model.AuthMethod{}, false
	}

	result := model.AuthMethod{
		Basic:        DecodeEndpointAuthMethodBasicPtr(x.Basic),
		APIKey:       DecodeEndpointAuthMethodAPIKeyPtr(x.ApiKey),
		JWT:          DecodeEndpointAuthMethodJWTPtr(x.Jwt),
		IPValidation: DecodeEndpointAuthMethodIPValidationPtr(x.IpValidation),
	}

	hasAny := result.Basic != nil || result.APIKey != nil || result.JWT != nil || result.IPValidation != nil
	return result, hasAny
}

func DecodeEndpointAuthMethodBasicPtr(x *ruto_v1.EndpointAuthMethodBasic) *model.AuthMethodBasic {
	if x == nil {
		return nil
	}
	return new(DecodeEndpointAuthMethodBasic(x))
}

func DecodeEndpointAuthMethodAPIKeyPtr(x *ruto_v1.EndpointAuthMethodAPIKey) *model.AuthMethodAPIKey {
	if x == nil {
		return nil
	}
	return new(DecodeEndpointAuthMethodAPIKey(x))
}

func DecodeEndpointAuthMethodJWTPtr(x *ruto_v1.EndpointAuthMethodJWT) *model.AuthMethodJWT {
	if x == nil {
		return nil
	}
	return new(DecodeEndpointAuthMethodJWT(x))
}

func DecodeEndpointAuthMethodIPValidationPtr(x *ruto_v1.EndpointAuthMethodIPValidation) *model.AuthMethodIPValidation {
	if x == nil {
		return nil
	}
	return new(DecodeEndpointAuthMethodIPValidation(x))
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

func EncodeEndpointAuthMethodIPValidation(x model.AuthMethodIPValidation) *ruto_v1.EndpointAuthMethodIPValidation {
	return &ruto_v1.EndpointAuthMethodIPValidation{
		AllowedIps: x.AllowedIps,
	}
}

func DecodeEndpointAuthMethodIPValidation(x *ruto_v1.EndpointAuthMethodIPValidation) model.AuthMethodIPValidation {
	if x == nil {
		return model.AuthMethodIPValidation{}
	}
	return model.AuthMethodIPValidation{
		AllowedIps: x.AllowedIps,
	}
}
