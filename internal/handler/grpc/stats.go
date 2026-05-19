package grpc

import (
	"context"

	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/rendau/ruto/internal/handler/grpc/dto"
	usecase "github.com/rendau/ruto/internal/usecase/stats"
	"github.com/rendau/ruto/pkg/proto/ruto_v1"
)

type Stats struct {
	ruto_v1.UnsafeStatsServer
	usecase *usecase.Usecase
}

func NewStats(usecase *usecase.Usecase) *Stats {
	return &Stats{usecase: usecase}
}

func (h *Stats) Get(ctx context.Context, _ *emptypb.Empty) (*ruto_v1.StatsResponse, error) {
	result, err := h.usecase.Get(ctx)
	if err != nil {
		return nil, err
	}
	return dto.EncodeStatsResponse(result), nil
}
