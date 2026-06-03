package grpc

import (
	"context"

	"github.com/samber/lo"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/rendau/ruto/pkg/proto/ruto_v1"

	usecase "github.com/rendau/ruto/internal/usecase/app"

	"github.com/rendau/ruto/internal/handler/grpc/dto"
)

type App struct {
	ruto_v1.UnsafeAppServer
	usecase *usecase.Usecase
}

func NewApp(usecase *usecase.Usecase) *App { return &App{usecase: usecase} }

func (h *App) List(ctx context.Context, req *ruto_v1.AppListReq) (*ruto_v1.AppListRep, error) {
	if req.ListParams == nil {
		req.ListParams = &ruto_v1.ListParamsSt{}
	}
	items, tCount, err := h.usecase.List(ctx, dto.DecodeAppListReq(req))
	if err != nil {
		return nil, err
	}
	return &ruto_v1.AppListRep{
		PaginationInfo: &ruto_v1.PaginationInfoSt{
			Page:       req.ListParams.Page,
			PageSize:   req.ListParams.PageSize,
			TotalCount: tCount,
		},
		Results: lo.Map(items, dto.EncodeAppMain),
	}, nil
}

func (h *App) Get(ctx context.Context, req *ruto_v1.AppGetReq) (*ruto_v1.AppMain, error) {
	item, err := h.usecase.Get(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return dto.EncodeAppMain(item, 0), nil
}

func (h *App) Create(ctx context.Context, req *ruto_v1.AppMain) (*ruto_v1.AppCreateRep, error) {
	newId, err := h.usecase.Create(ctx, dto.DecodeAppMain(req))
	if err != nil {
		return nil, err
	}
	return &ruto_v1.AppCreateRep{Id: newId}, nil
}

func (h *App) Update(ctx context.Context, req *ruto_v1.AppMain) (*emptypb.Empty, error) {
	if err := h.usecase.Update(ctx, req.Id, dto.DecodeAppMain(req)); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (h *App) Delete(ctx context.Context, req *ruto_v1.AppGetReq) (*emptypb.Empty, error) {
	if err := h.usecase.Delete(ctx, req.Id); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (h *App) GetSwaggerEndpointsDiff(ctx context.Context, req *ruto_v1.AppGetReq) (*ruto_v1.AppSwaggerEndpointsDiffRep, error) {
	rep, err := h.usecase.GetSwaggerEndpointsDiff(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return dto.EncodeSwaggerEndpointDiff(rep), nil
}

func (h *App) GetGrpcReflectionEndpoints(ctx context.Context, req *ruto_v1.AppGetReq) (*ruto_v1.AppGrpcReflectionEndpointsRep, error) {
	items, err := h.usecase.GetGrpcReflectionEndpoints(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return dto.EncodeGrpcReflectionEndpoints(items), nil
}

func (h *App) GetSwaggerUrlByBackendUrl(
	ctx context.Context,
	req *ruto_v1.AppGetSwaggerUrlByBackendUrlReq,
) (*ruto_v1.AppGetSwaggerUrlByBackendUrlRep, error) {
	swaggerURL, err := h.usecase.GetSwaggerURLByBackendURL(ctx, req.GetBackendUrl())
	if err != nil {
		return nil, err
	}

	return &ruto_v1.AppGetSwaggerUrlByBackendUrlRep{
		SwaggerUrl: swaggerURL,
	}, nil
}

func (h *App) GetVariablesEffective(ctx context.Context, req *ruto_v1.AppVariablesEffectiveReq) (*ruto_v1.VariablesEffectiveRep, error) {
	items, err := h.usecase.GetVariablesEffective(ctx, req.GetId(), lo.FilterMap(req.GetVariables(), dto.DecodeVariable))
	if err != nil {
		return nil, err
	}
	return dto.EncodeVariablesEffectiveRep(items), nil
}
