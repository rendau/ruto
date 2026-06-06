package grpc

import (
	"context"

	"github.com/samber/lo"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/structpb"

	varsModel "github.com/rendau/ruto/internal/domain/vars/model"
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
		Results: lo.Map(items, dto.EncodeEndpointData),
	}, nil
}

func (h *Endpoint) Get(ctx context.Context, req *ruto_v1.EndpointGetReq) (*structpb.Struct, error) {
	item, err := h.usecase.Get(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return dto.EncodeEndpointData(item, 0), nil
}

func (h *Endpoint) Interpolate(ctx context.Context, req *ruto_v1.EndpointInterpolateReq) (*structpb.Struct, error) {
	item, err := h.usecase.Interpolate(ctx, req.GetId(), varsModel.Vars(req.GetVariables()))
	if err != nil {
		return nil, err
	}
	return dto.EncodeEndpointData(item, 0), nil
}

func (h *Endpoint) Inherited(ctx context.Context, req *ruto_v1.EndpointInheritedReq) (*structpb.Struct, error) {
	item, err := h.usecase.Inherited(ctx, req.GetId(), varsModel.Vars(req.GetVariables()))
	if err != nil {
		return nil, err
	}
	return dto.EncodeEndpointData(item, 0), nil
}

func (h *Endpoint) TestRequest(ctx context.Context, req *ruto_v1.EndpointTestReq) (*ruto_v1.EndpointTestRep, error) {
	result, err := h.usecase.TestRequest(
		ctx,
		req.GetId(),
		dto.DecodeEndpointTestKVs(req.GetPathParams()),
		dto.DecodeEndpointTestKVs(req.GetQueryParams()),
		req.GetBody(),
	)
	if err != nil {
		return nil, err
	}
	return dto.EncodeEndpointTestResult(result), nil
}

func (h *Endpoint) Create(ctx context.Context, req *structpb.Struct) (*ruto_v1.EndpointCreateRep, error) {
	newId, err := h.usecase.Create(ctx, dto.DecodeEndpointData(req))
	if err != nil {
		return nil, err
	}
	return &ruto_v1.EndpointCreateRep{Id: newId}, nil
}

func (h *Endpoint) Update(ctx context.Context, req *ruto_v1.EndpointUpdateReq) (*emptypb.Empty, error) {
	if err := h.usecase.Update(ctx, req.Id, dto.DecodeEndpointData(req.GetData())); err != nil {
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
