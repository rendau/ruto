package grpc

import (
	"context"

	"github.com/samber/lo"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/rendau/ruto/pkg/proto/ruto_v1"

	usecase "github.com/rendau/ruto/internal/usecase/endpoint"

	"github.com/rendau/ruto/internal/handler/grpc/dto"
)

type Endpoint struct {
	ruto_v1.UnsafeEndpointServer
	usecase *usecase.Usecase
}

func NewEndpoint(usecase *usecase.Usecase) *Endpoint { return &Endpoint{usecase: usecase} }

func (h *Endpoint) List(ctx context.Context, req *ruto_v1.EndpointListReq) (*ruto_v1.EndpointListRep, error) {
	if req.ListParams == nil {
		req.ListParams = &ruto_v1.ListParamsSt{}
	}
	items, tCount, err := h.usecase.List(ctx, dto.DecodeEndpointListReq(req))
	if err != nil {
		return nil, err
	}
	return &ruto_v1.EndpointListRep{
		PaginationInfo: &ruto_v1.PaginationInfoSt{
			Page:       req.ListParams.Page,
			PageSize:   req.ListParams.PageSize,
			TotalCount: tCount,
		},
		Results: lo.Map(items, dto.EncodeEndpointMain),
	}, nil
}

func (h *Endpoint) Get(ctx context.Context, req *ruto_v1.EndpointGetReq) (*ruto_v1.EndpointMain, error) {
	item, err := h.usecase.Get(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return dto.EncodeEndpointMain(item, 0), nil
}

func (h *Endpoint) Create(ctx context.Context, req *ruto_v1.EndpointMain) (*ruto_v1.EndpointCreateRep, error) {
	newId, err := h.usecase.Create(ctx, dto.DecodeEndpointMain(req))
	if err != nil {
		return nil, err
	}
	return &ruto_v1.EndpointCreateRep{Id: newId}, nil
}

func (h *Endpoint) Update(ctx context.Context, req *ruto_v1.EndpointMain) (*emptypb.Empty, error) {
	if err := h.usecase.Update(ctx, req.Id, dto.DecodeEndpointMain(req)); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (h *Endpoint) Delete(ctx context.Context, req *ruto_v1.EndpointGetReq) (*emptypb.Empty, error) {
	if err := h.usecase.Delete(ctx, req.Id); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}
