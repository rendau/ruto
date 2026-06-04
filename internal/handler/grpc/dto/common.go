package dto

import (
	"github.com/goccy/go-json"
	"google.golang.org/protobuf/types/known/structpb"

	commonModel "github.com/rendau/ruto/internal/domain/common/model"
	"github.com/rendau/ruto/pkg/proto/ruto_v1"
)

func DecodeListParams(listParams *ruto_v1.ListParamsSt) commonModel.ListParams {
	if listParams == nil {
		return commonModel.ListParams{}
	}

	return commonModel.ListParams{
		Page:           listParams.Page,
		PageSize:       listParams.PageSize,
		WithTotalCount: listParams.WithTotalCount,
		OnlyCount:      listParams.OnlyCount,
		SortName:       listParams.SortName,
		Sort:           listParams.Sort,
	}
}

func JsonObjToGrpcStruct(v []byte) *structpb.Struct {
	var result *structpb.Struct

	if len(v) == 0 {
		return nil
	}

	obj := map[string]any{}

	if err := json.Unmarshal(v, &obj); err == nil {
		result, err = structpb.NewStruct(obj)
		if err != nil {
			result = nil
		}
	}

	return result
}

func GrpcStructToJsonObj(v *structpb.Struct) []byte {
	if v == nil {
		return nil
	}

	result, err := json.Marshal(v.AsMap())
	if err != nil {
		return nil
	}

	return result
}

func DomainToGrpcStruct(v any) *structpb.Struct {
	result, err := json.Marshal(v)
	if err != nil {
		return nil
	}

	return JsonObjToGrpcStruct(result)
}

func GrpcStructToDomain[T any](v *structpb.Struct) *T {
	var result T
	if v == nil {
		return nil
	}

	raw := GrpcStructToJsonObj(v)
	if len(raw) == 0 {
		return nil
	}

	if err := json.Unmarshal(raw, &result); err != nil {
		return nil
	}

	return &result
}
