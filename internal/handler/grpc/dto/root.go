package dto

import (
	"github.com/samber/lo"

	domModel "github.com/rendau/ruto/internal/domain/root/model"

	"github.com/rendau/ruto/pkg/proto/ruto_v1"
)

func EncodeRootMain(v *domModel.Main, _ int) *ruto_v1.RootMain {
	if v == nil {
		return nil
	}

	return &ruto_v1.RootMain{
		PublicBaseUrl: v.PublicBaseUrl,
		Cors:          EncodeRootCors(v.Cors),
		Jwt:           lo.Map(v.Jwt, EncodeRootJwt),
	}
}

func DecodeRootSetReq(v *ruto_v1.RootSetReq) *domModel.Edit {
	if v == nil {
		return nil
	}

	result := &domModel.Edit{
		PublicBaseUrl: v.PublicBaseUrl,
		Cors:          DecodeRootCors(v.Cors),
	}

	if v.Jwt != nil {
		result.Jwt = new(lo.Map(v.Jwt, DecodeRootJwt))
	}

	return result
}

func EncodeRootCors(x *domModel.Cors) *ruto_v1.RootCors {
	if x == nil {
		return nil
	}
	return &ruto_v1.RootCors{
		Enabled:          x.Enabled,
		AllowCredentials: x.AllowCredentials,
		MaxAge:           x.MaxAge,
		AllowOrigins:     x.AllowOrigins,
		AllowMethods:     x.AllowMethods,
		AllowHeaders:     x.AllowHeaders,
	}
}

func DecodeRootCors(x *ruto_v1.RootCors) *domModel.Cors {
	if x == nil {
		return nil
	}
	return &domModel.Cors{
		Enabled:          x.Enabled,
		AllowCredentials: x.AllowCredentials,
		MaxAge:           x.MaxAge,
		AllowOrigins:     x.AllowOrigins,
		AllowMethods:     x.AllowMethods,
		AllowHeaders:     x.AllowHeaders,
	}
}

func EncodeRootJwt(x *domModel.Jwt, _ int) *ruto_v1.RootJwt {
	if x == nil {
		return nil
	}
	return &ruto_v1.RootJwt{
		JwkUrl: x.JwkUrl,
	}
}

func DecodeRootJwt(x *ruto_v1.RootJwt, _ int) *domModel.Jwt {
	if x == nil {
		return nil
	}
	return &domModel.Jwt{
		JwkUrl: x.JwkUrl,
	}
}
