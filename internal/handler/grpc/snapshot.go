package grpc

import (
	"context"

	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/rendau/ruto/internal/handler/grpc/dto"
	usecase "github.com/rendau/ruto/internal/usecase/snapshot"
	"github.com/rendau/ruto/pkg/proto/ruto_v1"
)

type Snapshot struct {
	ruto_v1.UnsafeSnapshotServer
	usecase *usecase.Usecase
}

func NewSnapshot(usecase *usecase.Usecase) *Snapshot {
	return &Snapshot{usecase: usecase}
}

func (h *Snapshot) Get(ctx context.Context, _ *emptypb.Empty) (*structpb.Struct, error) {
	result, err := h.usecase.Get(ctx)
	if err != nil {
		return nil, err
	}
	return dto.JsonObjToGrpcStruct(result), nil
}
