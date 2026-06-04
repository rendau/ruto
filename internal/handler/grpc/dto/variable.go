package dto

import (
	"github.com/samber/lo"
	"google.golang.org/protobuf/types/known/structpb"

	variableModel "github.com/rendau/ruto/internal/domain/variable/model"
	"github.com/rendau/ruto/pkg/proto/ruto_v1"
)

func EncodeVariable(v variableModel.Variable, _ int) *structpb.Struct {
	return DomainToGrpcStruct(v)
}

func DecodeVariable(v *structpb.Struct, _ int) (variableModel.Variable, bool) {
	if v == nil {
		return variableModel.Variable{}, false
	}

	result := GrpcStructToDomain[variableModel.Variable](v)
	if result == nil {
		return variableModel.Variable{}, false
	}

	return *result, true
}

func EncodeVariablesEffectiveRep(items []variableModel.Variable) *ruto_v1.VariablesEffectiveRep {
	return &ruto_v1.VariablesEffectiveRep{
		Variables: lo.Map(items, EncodeVariable),
	}
}
