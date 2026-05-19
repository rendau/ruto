package grpc

import (
	"context"

	"google.golang.org/protobuf/types/known/emptypb"

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

func (h *Snapshot) GetVersion(ctx context.Context, _ *emptypb.Empty) (*ruto_v1.SnapshotVersion, error) {
	result := h.usecase.GetVersion()
	return &ruto_v1.SnapshotVersion{
		Version: result,
	}, nil
}

func (h *Snapshot) Get(_ context.Context, _ *emptypb.Empty) (*ruto_v1.SnapshotResponse, error) {
	result := h.usecase.Get()
	return &ruto_v1.SnapshotResponse{
		Data: dto.JsonObjToGrpcStruct(result),
	}, nil
}

func (h *Snapshot) Deploy(_ context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	h.usecase.Refresh()
	return &emptypb.Empty{}, nil
}
