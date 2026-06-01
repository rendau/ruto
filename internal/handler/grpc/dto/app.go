package dto

import (
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
		GrpcPort:   uint32(v.GrpcPort),
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
		GrpcPort:   int(v.GrpcPort),
	}
}

// AppBackend

func EncodeAppBackend(x model.AppBackend) *ruto_v1.AppBackend {
	return &ruto_v1.AppBackend{
		Url:        x.Url,
		SwaggerUrl: x.SwaggerUrl,
	}
}

func DecodeAppBackend(x *ruto_v1.AppBackend) model.AppBackend {
	if x == nil {
		return model.AppBackend{}
	}
	return model.AppBackend{
		Url:        x.Url,
		SwaggerUrl: x.SwaggerUrl,
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
	result := make([]*ruto_v1.AppSwaggerEndpoint, 0, len(items))
	for _, item := range items {
		result = append(result, &ruto_v1.AppSwaggerEndpoint{
			Method: item.Method,
			Path:   item.Path,
		})
	}
	return result
}
