package grpc

import (
	"context"

	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/rendau/ruto/internal/handler/grpc/dto"
	usecase "github.com/rendau/ruto/internal/usecase/gateway"
	"github.com/rendau/ruto/pkg/proto/ruto_v1"
)

type Gateway struct {
	ruto_v1.UnsafeGatewayServer
	usecase *usecase.Usecase
}

func NewGateway(usecase *usecase.Usecase) *Gateway {
	return &Gateway{usecase: usecase}
}

func (h *Gateway) Heartbeat(ctx context.Context, req *ruto_v1.GatewayHeartbeatRequest) (*emptypb.Empty, error) {
	err := h.usecase.Heartbeat(ctx, dto.DecodeGatewayHeartbeatReq(req))
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (h *Gateway) List(ctx context.Context, _ *emptypb.Empty) (*ruto_v1.GatewayListResponse, error) {
	items, err := h.usecase.List(ctx)
	if err != nil {
		return nil, err
	}
	return dto.EncodeGatewayListResponse(items), nil
}
