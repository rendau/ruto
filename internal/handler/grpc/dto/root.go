package dto

import (
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/rendau/ruto/internal/domain/root/model"
)

func EncodeRootData(v *model.Root, _ int) *structpb.Struct {
	return DomainToGrpcStruct(v)
}

func DecodeRootData(v *structpb.Struct) *model.Root {
	return GrpcStructToDomain[model.Root](v)
}
