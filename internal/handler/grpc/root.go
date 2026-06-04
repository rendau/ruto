package grpc

import (
	"context"

	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/structpb"

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

func (h *Root) Get(ctx context.Context, _ *emptypb.Empty) (*structpb.Struct, error) {
	item, err := h.usecase.Get(ctx)
	if err != nil {
		return nil, err
	}
	return dto.EncodeRootData(item, 0), nil
}

func (h *Root) Set(ctx context.Context, req *structpb.Struct) (*emptypb.Empty, error) {
	if err := h.usecase.Set(ctx, dto.DecodeRootData(req)); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (h *Root) Interpolate(ctx context.Context, req *ruto_v1.RootInterpolateReq) (*structpb.Struct, error) {
	item, err := h.usecase.Interpolate(ctx, req.GetVariables())
	if err != nil {
		return nil, err
	}
	return dto.EncodeRootData(item, 0), nil
}

func (h *Root) GetJwtKidsByUrls(ctx context.Context, req *ruto_v1.RootJwtKidsReq) (*ruto_v1.RootJwtKidsRep, error) {
	kids, err := h.usecase.GetJwtKidsByURLs(ctx, req.GetUrls())
	if err != nil {
		return nil, err
	}
	return &ruto_v1.RootJwtKidsRep{Kids: kids}, nil
}
