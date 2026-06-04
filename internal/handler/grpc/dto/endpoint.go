package dto

import (
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/rendau/ruto/pkg/proto/ruto_v1"

	"github.com/rendau/ruto/internal/domain/endpoint/model"
)

func EncodeEndpointData(v *model.Endpoint, _ int) *structpb.Struct {
	return DomainToGrpcStruct(v)
}

func DecodeEndpointListReq(v *ruto_v1.EndpointListReq) *model.ListReq {
	return &model.ListReq{
		ListParams: DecodeListParams(v.ListParams),
		AppId:      v.AppId,
		Active:     v.Active,
	}
}

func DecodeEndpointData(v *structpb.Struct) *model.Endpoint {
	return GrpcStructToDomain[model.Endpoint](v)
}
