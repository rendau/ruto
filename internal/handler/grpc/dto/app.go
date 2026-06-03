package dto

import (
	"github.com/samber/lo"

	"github.com/rendau/ruto/pkg/proto/ruto_v1"

	"github.com/rendau/ruto/internal/domain/app/model"
	usecaseApp "github.com/rendau/ruto/internal/usecase/app"
)

func EncodeAppMain(v *model.App, _ int) *ruto_v1.AppMain {
	return &ruto_v1.AppMain{
		Id:         v.Id,
		Active:     v.Active,
		PathPrefix: v.PathPrefix,
		Name:       v.Name,
		Backend:    EncodeAppBackend(v.Backend),
		Auth:       EncodeEndpointAuth(v.Auth),
	}
}

func DecodeAppListReq(v *ruto_v1.AppListReq) *model.ListReq {
	return &model.ListReq{
		ListParams: DecodeListParams(v.ListParams),
		Active:     v.Active,
	}
}

func DecodeAppMain(v *ruto_v1.AppMain) *model.App {
	return &model.App{
		Id:         v.Id,
		Active:     v.Active,
		PathPrefix: v.PathPrefix,
		Name:       v.Name,
		Backend:    DecodeAppBackend(v.Backend),
		Auth:       DecodeEndpointAuth(v.Auth),
	}
}

// AppBackend

func EncodeAppBackend(x model.AppBackend) *ruto_v1.AppBackend {
	return &ruto_v1.AppBackend{
		Url:        x.Url,
		SwaggerUrl: x.SwaggerUrl,
		GrpcPort:   uint32(x.GrpcPort),
	}
}

func DecodeAppBackend(x *ruto_v1.AppBackend) model.AppBackend {
	if x == nil {
		return model.AppBackend{}
	}
	return model.AppBackend{
		Url:        x.Url,
		SwaggerUrl: x.SwaggerUrl,
		GrpcPort:   int(x.GrpcPort),
	}
}

func EncodeSwaggerEndpointDiff(x *usecaseApp.SwaggerEndpointsDiff) *ruto_v1.AppSwaggerEndpointsDiffRep {
	if x == nil {
		return &ruto_v1.AppSwaggerEndpointsDiffRep{}
	}
	return &ruto_v1.AppSwaggerEndpointsDiffRep{
		Unregistered:      loMapSwaggerEndpoints(x.Unregistered),
		RegisteredInvalid: loMapSwaggerEndpoints(x.RegisteredInvalid),
	}
}

func loMapSwaggerEndpoints(items []usecaseApp.SwaggerEndpoint) []*ruto_v1.AppSwaggerEndpoint {
	return lo.Map(items, func(item usecaseApp.SwaggerEndpoint, _ int) *ruto_v1.AppSwaggerEndpoint {
		return &ruto_v1.AppSwaggerEndpoint{
			Method: item.Method,
			Path:   item.Path,
		}
	})
}

func EncodeGrpcReflectionEndpoints(items []usecaseApp.GrpcReflectionEndpoint) *ruto_v1.AppGrpcReflectionEndpointsRep {
	return &ruto_v1.AppGrpcReflectionEndpointsRep{
		Results: lo.Map(items, func(item usecaseApp.GrpcReflectionEndpoint, _ int) *ruto_v1.AppGrpcReflectionEndpoint {
			return &ruto_v1.AppGrpcReflectionEndpoint{
				Service: item.Service,
				Method:  item.Method,
				Path:    item.Path,
			}
		}),
	}
}
