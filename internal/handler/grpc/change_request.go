package grpc

import (
	"context"

	"github.com/samber/lo"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/rendau/ruto/internal/handler/grpc/dto"
	usecase "github.com/rendau/ruto/internal/usecase/changerequest"
	"github.com/rendau/ruto/pkg/proto/ruto_v1"
)

type ChangeRequest struct {
	ruto_v1.UnsafeChangeRequestServer
	usecase *usecase.Usecase
}

func NewChangeRequest(usecase *usecase.Usecase) *ChangeRequest {
	return &ChangeRequest{usecase: usecase}
}

func (h *ChangeRequest) List(ctx context.Context, req *ruto_v1.ChangeRequestListReq) (*ruto_v1.ChangeRequestListRep, error) {
	if req.ListParams == nil {
		req.ListParams = &ruto_v1.ListParamsSt{}
	}
	items, tCount, err := h.usecase.List(ctx, dto.DecodeChangeRequestListReq(req))
	if err != nil {
		return nil, err
	}
	return &ruto_v1.ChangeRequestListRep{
		PaginationInfo: &ruto_v1.PaginationInfoSt{
			Page:       req.ListParams.Page,
			PageSize:   req.ListParams.PageSize,
			TotalCount: tCount,
		},
		Results: lo.Map(items, dto.EncodeChangeRequestMain),
	}, nil
}

func (h *ChangeRequest) Get(ctx context.Context, req *ruto_v1.ChangeRequestGetReq) (*ruto_v1.ChangeRequestDetail, error) {
	item, err := h.usecase.Get(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return dto.EncodeChangeRequestDetail(item), nil
}

func (h *ChangeRequest) Apply(ctx context.Context, req *ruto_v1.ChangeRequestApplyReq) (*emptypb.Empty, error) {
	if err := h.usecase.Apply(ctx, req.Id, req.Publish); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (h *ChangeRequest) Reject(ctx context.Context, req *ruto_v1.ChangeRequestRejectReq) (*emptypb.Empty, error) {
	if err := h.usecase.Reject(ctx, req.Id, req.Comment); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}
