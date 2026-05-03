package grpc

import (
	"context"

	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/rendau/ruto/internal/handler/grpc/dto"
	usecase "github.com/rendau/ruto/internal/usecase/root"
	"github.com/rendau/ruto/pkg/proto/ruto_v1"
)

type Root struct {
	ruto_v1.UnsafeRootServer
	usecase *usecase.Usecase
}

func NewRoot(usecase *usecase.Usecase) *Root {
	return &Root{usecase: usecase}
}

func (h *Root) Get(ctx context.Context, _ *emptypb.Empty) (*ruto_v1.RootMain, error) {
	item, err := h.usecase.Get(ctx)
	if err != nil {
		return nil, err
	}
	return dto.EncodeRootMain(item, 0), nil
}

func (h *Root) Set(ctx context.Context, req *ruto_v1.RootSetReq) (*emptypb.Empty, error) {
	if err := h.usecase.Set(ctx, dto.DecodeRootSetReq(req)); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}
