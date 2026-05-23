package grpc

import (
	"context"
	"fmt"

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
	result, err := h.usecase.GetVersion(ctx)
	if err != nil {
		return nil, fmt.Errorf("usecase.GetVersion: %w", err)
	}
	return &ruto_v1.SnapshotVersion{
		Version: result,
	}, nil
}

func (h *Snapshot) Get(ctx context.Context, _ *emptypb.Empty) (*ruto_v1.SnapshotResponse, error) {
	result, err := h.usecase.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("usecase.Get: %w", err)
	}
	return &ruto_v1.SnapshotResponse{
		Data: dto.JsonObjToGrpcStruct(result),
	}, nil
}

func (h *Snapshot) Deploy(ctx context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	if err := h.usecase.Refresh(ctx); err != nil {
		return nil, fmt.Errorf("usecase.Refresh: %w", err)
	}
	return &emptypb.Empty{}, nil
}
