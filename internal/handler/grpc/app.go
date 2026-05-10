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

func (h *App) Create(ctx context.Context, req *ruto_v1.AppCreateReq) (*ruto_v1.AppCreateRep, error) {
	newId, err := h.usecase.Create(ctx, dto.DecodeAppCreateReq(req))
	if err != nil {
		return nil, err
	}
	return &ruto_v1.AppCreateRep{Id: newId}, nil
}

func (h *App) Update(ctx context.Context, req *ruto_v1.AppUpdateReq) (*emptypb.Empty, error) {
	if err := h.usecase.Update(ctx, req.Id, dto.DecodeAppUpdateReq(req)); err != nil {
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
