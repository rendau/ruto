package grpc

import (
	"context"

	"github.com/samber/lo"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/rendau/ruto/internal/handler/grpc/dto"
	usecase "github.com/rendau/ruto/internal/usecase/seenpath"
	"github.com/rendau/ruto/pkg/proto/ruto_v1"
)

type SeenPath struct {
	ruto_v1.UnsafeSeenPathServer
	usecase *usecase.Usecase
}

func NewSeenPath(usecase *usecase.Usecase) *SeenPath {
	return &SeenPath{usecase: usecase}
}

func (h *SeenPath) List(ctx context.Context, req *ruto_v1.SeenPathListReq) (*ruto_v1.SeenPathListRep, error) {
	items, err := h.usecase.List(ctx, req.AppId)
	if err != nil {
		return nil, err
	}
	return &ruto_v1.SeenPathListRep{
		Results: lo.Map(items, dto.EncodeSeenPathMain),
	}, nil
}

func (h *SeenPath) Clear(ctx context.Context, req *ruto_v1.SeenPathClearReq) (*emptypb.Empty, error) {
	if err := h.usecase.Clear(ctx, req.AppId); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}
