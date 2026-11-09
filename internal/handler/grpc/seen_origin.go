package grpc

import (
	"context"

	"github.com/samber/lo"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/rendau/ruto/internal/handler/grpc/dto"
	usecase "github.com/rendau/ruto/internal/usecase/seenorigin"
	"github.com/rendau/ruto/pkg/proto/ruto_v1"
)

type SeenOrigin struct {
	ruto_v1.UnsafeSeenOriginServer
	usecase *usecase.Usecase
}

func NewSeenOrigin(usecase *usecase.Usecase) *SeenOrigin {
	return &SeenOrigin{usecase: usecase}
}

func (h *SeenOrigin) List(ctx context.Context, _ *ruto_v1.SeenOriginListReq) (*ruto_v1.SeenOriginListRep, error) {
	items, err := h.usecase.List(ctx)
	if err != nil {
		return nil, err
	}
	return &ruto_v1.SeenOriginListRep{
		Results: lo.Map(items, dto.EncodeSeenOriginMain),
	}, nil
}

func (h *SeenOrigin) Delete(ctx context.Context, req *ruto_v1.SeenOriginDeleteReq) (*emptypb.Empty, error) {
	if err := h.usecase.Delete(ctx, req.Origins); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (h *SeenOrigin) Clear(ctx context.Context, _ *ruto_v1.SeenOriginClearReq) (*emptypb.Empty, error) {
	if err := h.usecase.Clear(ctx); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}
