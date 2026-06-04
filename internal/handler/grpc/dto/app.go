package dto

import (
	"github.com/samber/lo"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/rendau/ruto/pkg/proto/ruto_v1"

	"github.com/rendau/ruto/internal/domain/app/model"
	usecaseApp "github.com/rendau/ruto/internal/usecase/app"
)

func EncodeAppData(v *model.App, _ int) *structpb.Struct {
	return DomainToGrpcStruct(v)
}

func DecodeAppListReq(v *ruto_v1.AppListReq) *model.ListReq {
	return &model.ListReq{
		ListParams: DecodeListParams(v.ListParams),
		Active:     v.Active,
	}
}

func DecodeAppData(v *structpb.Struct) *model.App {
	return GrpcStructToDomain[model.App](v)
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
