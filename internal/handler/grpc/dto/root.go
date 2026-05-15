package dto

import (
	"github.com/samber/lo"

	"github.com/rendau/ruto/internal/domain/root/model"

	"github.com/rendau/ruto/pkg/proto/ruto_v1"
)

func EncodeRootMain(v *model.Root, _ int) *ruto_v1.RootMain {
	return &ruto_v1.RootMain{
		BaseUrl: v.BaseUrl,
		Cors:    EncodeRootCors(v.Cors),
		Jwt:     lo.Map(v.Jwt, EncodeRootJwt),
		Auth:    EncodeEndpointAuth(v.Auth),
	}
}

func DecodeRootMain(v *ruto_v1.RootMain) *model.Root {
	return &model.Root{
		BaseUrl: v.BaseUrl,
		Cors:    DecodeRootCors(v.Cors),
		Jwt:     lo.FilterMap(v.Jwt, DecodeRootJwt),
		Auth:    DecodeEndpointAuth(v.Auth),
	}
}

func EncodeRootCors(x model.RootCors) *ruto_v1.RootCors {
	return &ruto_v1.RootCors{
		Enabled:          x.Enabled,
		AllowCredentials: x.AllowCredentials,
		MaxAge:           x.MaxAge,
		AllowOrigins:     x.AllowOrigins,
		AllowMethods:     x.AllowMethods,
		AllowHeaders:     x.AllowHeaders,
	}
}

func DecodeRootCors(x *ruto_v1.RootCors) model.RootCors {
	if x == nil {
		return model.RootCors{}
	}
	return model.RootCors{
		Enabled:          x.Enabled,
		AllowCredentials: x.AllowCredentials,
		MaxAge:           x.MaxAge,
		AllowOrigins:     x.AllowOrigins,
		AllowMethods:     x.AllowMethods,
		AllowHeaders:     x.AllowHeaders,
	}
}

func EncodeRootJwt(x model.RootJwt, _ int) *ruto_v1.RootJwt {
	return &ruto_v1.RootJwt{
		JwkUrl: x.JwkUrl,
	}
}

func DecodeRootJwt(x *ruto_v1.RootJwt, _ int) (model.RootJwt, bool) {
	if x == nil {
		return model.RootJwt{}, false
	}
	return model.RootJwt{
		JwkUrl: x.JwkUrl,
	}, true
}
