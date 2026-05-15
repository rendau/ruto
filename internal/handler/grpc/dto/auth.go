package dto

import (
	"github.com/samber/lo"

	authModel "github.com/rendau/ruto/internal/domain/auth/model"
	"github.com/rendau/ruto/pkg/proto/ruto_v1"
)

func EncodeEndpointAuth(x authModel.Auth) *ruto_v1.Auth {
	return &ruto_v1.Auth{
		Enabled: x.Enabled,
		Mode:    x.Mode,
		Methods: lo.Map(x.Methods, EncodeEndpointAuthMethod),
	}
}

func DecodeEndpointAuth(x *ruto_v1.Auth) authModel.Auth {
	if x == nil {
		return authModel.Auth{}
	}
	return authModel.Auth{
		Enabled: x.Enabled,
		Mode:    x.Mode,
		Methods: lo.FilterMap(x.Methods, DecodeEndpointAuthMethod),
	}
}

func EncodeEndpointAuthMethod(x authModel.AuthMethod, _ int) *ruto_v1.AuthMethod {
	result := &ruto_v1.AuthMethod{
		Basic:        EncodeEndpointAuthMethodBasicPtr(x.Basic),
		ApiKey:       EncodeEndpointAuthMethodAPIKeyPtr(x.APIKey),
		Jwt:          EncodeEndpointAuthMethodJWTPtr(x.JWT),
		IpValidation: EncodeEndpointAuthMethodIPValidationPtr(x.IPValidation),
	}
	return result
}

func EncodeEndpointAuthMethodBasicPtr(x *authModel.AuthMethodBasic) *ruto_v1.AuthMethodBasic {
	if x != nil {
		return EncodeEndpointAuthMethodBasic(*x)
	}
	return nil
}

func EncodeEndpointAuthMethodAPIKeyPtr(x *authModel.AuthMethodAPIKey) *ruto_v1.AuthMethodAPIKey {
	if x != nil {
		return EncodeEndpointAuthMethodAPIKey(*x)
	}
	return nil
}

func EncodeEndpointAuthMethodJWTPtr(x *authModel.AuthMethodJWT) *ruto_v1.AuthMethodJWT {
	if x != nil {
		return EncodeEndpointAuthMethodJWT(*x)
	}
	return nil
}

func EncodeEndpointAuthMethodIPValidationPtr(x *authModel.AuthMethodIPValidation) *ruto_v1.AuthMethodIPValidation {
	if x != nil {
		return EncodeEndpointAuthMethodIPValidation(*x)
	}
	return nil
}

func DecodeEndpointAuthMethod(x *ruto_v1.AuthMethod, _ int) (authModel.AuthMethod, bool) {
	if x == nil {
		return authModel.AuthMethod{}, false
	}

	result := authModel.AuthMethod{
		Basic:        DecodeEndpointAuthMethodBasicPtr(x.Basic),
		APIKey:       DecodeEndpointAuthMethodAPIKeyPtr(x.ApiKey),
		JWT:          DecodeEndpointAuthMethodJWTPtr(x.Jwt),
		IPValidation: DecodeEndpointAuthMethodIPValidationPtr(x.IpValidation),
	}

	hasAny := result.Basic != nil || result.APIKey != nil || result.JWT != nil || result.IPValidation != nil
	return result, hasAny
}

func DecodeEndpointAuthMethodBasicPtr(x *ruto_v1.AuthMethodBasic) *authModel.AuthMethodBasic {
	if x == nil {
		return nil
	}
	return new(DecodeEndpointAuthMethodBasic(x))
}

func DecodeEndpointAuthMethodAPIKeyPtr(x *ruto_v1.AuthMethodAPIKey) *authModel.AuthMethodAPIKey {
	if x == nil {
		return nil
	}
	return new(DecodeEndpointAuthMethodAPIKey(x))
}

func DecodeEndpointAuthMethodJWTPtr(x *ruto_v1.AuthMethodJWT) *authModel.AuthMethodJWT {
	if x == nil {
		return nil
	}
	return new(DecodeEndpointAuthMethodJWT(x))
}

func DecodeEndpointAuthMethodIPValidationPtr(x *ruto_v1.AuthMethodIPValidation) *authModel.AuthMethodIPValidation {
	if x == nil {
		return nil
	}
	return new(DecodeEndpointAuthMethodIPValidation(x))
}

func EncodeEndpointAuthMethodBasic(x authModel.AuthMethodBasic) *ruto_v1.AuthMethodBasic {
	return &ruto_v1.AuthMethodBasic{
		Users: lo.Map(x.Users, EncodeEndpointAuthMethodBasicUser),
	}
}

func DecodeEndpointAuthMethodBasic(x *ruto_v1.AuthMethodBasic) authModel.AuthMethodBasic {
	if x == nil {
		return authModel.AuthMethodBasic{}
	}
	return authModel.AuthMethodBasic{
		Users: lo.FilterMap(x.Users, DecodeEndpointAuthMethodBasicUser),
	}
}

func EncodeEndpointAuthMethodBasicUser(x authModel.AuthMethodBasicUser, _ int) *ruto_v1.AuthMethodBasicUser {
	return &ruto_v1.AuthMethodBasicUser{
		Username: x.Username,
		Password: x.Password,
	}
}

func DecodeEndpointAuthMethodBasicUser(x *ruto_v1.AuthMethodBasicUser, _ int) (authModel.AuthMethodBasicUser, bool) {
	if x == nil {
		return authModel.AuthMethodBasicUser{}, false
	}
	return authModel.AuthMethodBasicUser{
		Username: x.Username,
		Password: x.Password,
	}, true
}

func EncodeEndpointAuthMethodAPIKey(x authModel.AuthMethodAPIKey) *ruto_v1.AuthMethodAPIKey {
	return &ruto_v1.AuthMethodAPIKey{
		Header: x.Header,
		Keys:   x.Keys,
	}
}

func DecodeEndpointAuthMethodAPIKey(x *ruto_v1.AuthMethodAPIKey) authModel.AuthMethodAPIKey {
	if x == nil {
		return authModel.AuthMethodAPIKey{}
	}
	return authModel.AuthMethodAPIKey{
		Header: x.Header,
		Keys:   x.Keys,
	}
}

func EncodeEndpointAuthMethodJWT(x authModel.AuthMethodJWT) *ruto_v1.AuthMethodJWT {
	return &ruto_v1.AuthMethodJWT{
		Kids:  x.Kids,
		Roles: x.Roles,
	}
}

func DecodeEndpointAuthMethodJWT(x *ruto_v1.AuthMethodJWT) authModel.AuthMethodJWT {
	if x == nil {
		return authModel.AuthMethodJWT{}
	}
	return authModel.AuthMethodJWT{
		Kids:  x.Kids,
		Roles: x.Roles,
	}
}

func EncodeEndpointAuthMethodIPValidation(x authModel.AuthMethodIPValidation) *ruto_v1.AuthMethodIPValidation {
	return &ruto_v1.AuthMethodIPValidation{
		AllowedIps: x.AllowedIps,
	}
}

func DecodeEndpointAuthMethodIPValidation(x *ruto_v1.AuthMethodIPValidation) authModel.AuthMethodIPValidation {
	if x == nil {
		return authModel.AuthMethodIPValidation{}
	}
	return authModel.AuthMethodIPValidation{
		AllowedIps: x.AllowedIps,
	}
}
