package dto

import (
	"sort"

	"github.com/samber/lo"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/rendau/ruto/pkg/proto/ruto_v1"

	"github.com/rendau/ruto/internal/domain/endpoint/model"
	varsModel "github.com/rendau/ruto/internal/domain/vars/model"
	usecase "github.com/rendau/ruto/internal/usecase/endpoint"
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

func DecodeEndpointTestKVs(items []*ruto_v1.EndpointTestKV) varsModel.Vars {
	return lo.SliceToMap(
		lo.Filter(items, func(item *ruto_v1.EndpointTestKV, _ int) bool {
			return item != nil && item.GetKey() != ""
		}),
		func(item *ruto_v1.EndpointTestKV) (string, string) {
			return item.GetKey(), item.GetValue()
		},
	)
}

func EncodeEndpointTestResult(v *usecase.TestRequestResult) *ruto_v1.EndpointTestRep {
	if v == nil {
		return &ruto_v1.EndpointTestRep{}
	}

	keys := lo.Keys(v.Headers)
	sort.Strings(keys)
	headers := lo.Map(keys, func(key string, _ int) *ruto_v1.EndpointTestKV {
		return &ruto_v1.EndpointTestKV{Key: key, Value: v.Headers[key]}
	})

	return &ruto_v1.EndpointTestRep{
		RequestUrl:    v.RequestURL,
		RequestMethod: v.RequestMethod,
		StatusCode:    int32(v.StatusCode),
		Headers:       headers,
		Body:          v.Body,
		DurationMs:    v.DurationMs,
		Error:         v.Error,
	}
}
