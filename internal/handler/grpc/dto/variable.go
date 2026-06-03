package dto

import (
	"github.com/samber/lo"

	variableModel "github.com/rendau/ruto/internal/domain/variable/model"
	"github.com/rendau/ruto/pkg/proto/ruto_v1"
)

func EncodeVariable(v variableModel.Variable, _ int) *ruto_v1.Variable {
	return &ruto_v1.Variable{
		Key:   v.Key,
		Value: v.Value,
	}
}

func DecodeVariable(v *ruto_v1.Variable, _ int) (variableModel.Variable, bool) {
	if v == nil {
		return variableModel.Variable{}, false
	}
	return variableModel.Variable{
		Key:   v.Key,
		Value: v.Value,
	}, true
}

func EncodeVariablesEffectiveRep(items []variableModel.Variable) *ruto_v1.VariablesEffectiveRep {
	return &ruto_v1.VariablesEffectiveRep{
		Variables: lo.Map(items, EncodeVariable),
	}
}
