package grpc

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/rendau/ruto/internal/handler/grpc/dto"
	usecase "github.com/rendau/ruto/internal/usecase/gateway"
	"github.com/rendau/ruto/pkg/proto/ruto_v1"
)

const notificationTypeCheckVersion = "check_version"

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

// Subscribe holds a long-lived server stream open for one connected gateway.
// The usecase blocks here for the lifetime of the stream, pushing a
// notification through stream.Send whenever the gateway should re-check the
// snapshot version. It returns when the gateway disconnects (stream context
// done) or a send fails.
func (h *Gateway) Subscribe(req *ruto_v1.GatewaySubscribeRequest, stream grpc.ServerStreamingServer[ruto_v1.GatewayNotification]) error {
	gatewayID := ""
	if req != nil {
		gatewayID = req.GatewayId
	}

	return h.usecase.Subscribe(stream.Context(), gatewayID, func() error {
		return stream.Send(&ruto_v1.GatewayNotification{Type: notificationTypeCheckVersion})
	})
}
