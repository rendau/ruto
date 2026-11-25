package grpc

import (
	"context"

	"github.com/samber/lo"

	"github.com/rendau/ruto/internal/handler/grpc/dto"
	usecase "github.com/rendau/ruto/internal/usecase/clientstat"
	"github.com/rendau/ruto/pkg/proto/ruto_v1"
)

type ClientStat struct {
	ruto_v1.UnsafeClientStatServer
	usecase *usecase.Usecase
}

func NewClientStat(usecase *usecase.Usecase) *ClientStat {
	return &ClientStat{usecase: usecase}
}

func (h *ClientStat) Overview(ctx context.Context, req *ruto_v1.ClientStatOverviewReq) (*ruto_v1.ClientStatOverviewRep, error) {
	result, err := h.usecase.Overview(ctx, dto.DecodeClientStatRange(req.RangeSeconds))
	if err != nil {
		return nil, err
	}
	return dto.EncodeClientStatOverviewRep(result), nil
}

func (h *ClientStat) Top(ctx context.Context, req *ruto_v1.ClientStatTopReq) (*ruto_v1.ClientStatTopRep, error) {
	items, err := h.usecase.Top(ctx, &usecase.TopReq{
		Range: dto.DecodeClientStatRange(req.RangeSeconds),
		Kind:  req.Kind,
		Limit: req.Limit,
	})
	if err != nil {
		return nil, err
	}
	return &ruto_v1.ClientStatTopRep{Results: lo.Map(items, dto.EncodeClientStatItem)}, nil
}

func (h *ClientStat) Series(ctx context.Context, req *ruto_v1.ClientStatSeriesReq) (*ruto_v1.ClientStatSeriesRep, error) {
	result, err := h.usecase.Series(ctx, req.Ip, dto.DecodeClientStatRange(req.RangeSeconds))
	if err != nil {
		return nil, err
	}
	return dto.EncodeClientStatSeriesRep(result), nil
}
