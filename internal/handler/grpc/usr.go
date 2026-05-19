package grpc

import (
	"context"

	"github.com/samber/lo"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/rendau/ruto/internal/handler/grpc/dto"
	usecase "github.com/rendau/ruto/internal/usecase/usr"
	"github.com/rendau/ruto/pkg/proto/ruto_v1"
)

type Usr struct {
	ruto_v1.UnsafeUsrServer
	usecase *usecase.Usecase
}

func NewUsr(usecase *usecase.Usecase) *Usr { return &Usr{usecase: usecase} }

func (h *Usr) List(ctx context.Context, req *ruto_v1.UsrListReq) (*ruto_v1.UsrListRep, error) {
	if req.ListParams == nil {
		req.ListParams = &ruto_v1.ListParamsSt{}
	}
	items, tCount, err := h.usecase.List(ctx, dto.DecodeUsrListReq(req))
	if err != nil {
		return nil, err
	}
	return &ruto_v1.UsrListRep{
		PaginationInfo: &ruto_v1.PaginationInfoSt{
			Page:       req.ListParams.Page,
			PageSize:   req.ListParams.PageSize,
			TotalCount: tCount,
		},
		Results: lo.Map(items, dto.EncodeUsrMain),
	}, nil
}

func (h *Usr) Get(ctx context.Context, req *ruto_v1.UsrGetReq) (*ruto_v1.UsrMain, error) {
	item, err := h.usecase.Get(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return dto.EncodeUsrMain(item, 0), nil
}

func (h *Usr) Create(ctx context.Context, req *ruto_v1.UsrMain) (*ruto_v1.UsrCreateRep, error) {
	newId, err := h.usecase.Create(ctx, dto.DecodeUsrMain(req))
	if err != nil {
		return nil, err
	}
	return &ruto_v1.UsrCreateRep{Id: newId}, nil
}

func (h *Usr) Update(ctx context.Context, req *ruto_v1.UsrMain) (*emptypb.Empty, error) {
	if err := h.usecase.Update(ctx, req.Id, dto.DecodeUsrMain(req)); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (h *Usr) Delete(ctx context.Context, req *ruto_v1.UsrGetReq) (*emptypb.Empty, error) {
	if err := h.usecase.Delete(ctx, req.Id); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (h *Usr) Login(ctx context.Context, req *ruto_v1.UsrLoginReq) (*ruto_v1.UsrLoginRep, error) {
	jwtToken, err := h.usecase.Login(ctx, req.Username, req.Password)
	if err != nil {
		return nil, err
	}

	return &ruto_v1.UsrLoginRep{
		Jwt: jwtToken,
	}, nil
}

func (h *Usr) GetProfile(ctx context.Context, _ *emptypb.Empty) (*ruto_v1.UsrMain, error) {
	item, err := h.usecase.GetProfile(ctx)
	if err != nil {
		return nil, err
	}
	return dto.EncodeUsrMain(item, 0), nil
}
