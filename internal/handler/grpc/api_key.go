package grpc

import (
	"context"

	"github.com/samber/lo"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/rendau/ruto/internal/handler/grpc/dto"
	usecase "github.com/rendau/ruto/internal/usecase/apikey"
	"github.com/rendau/ruto/pkg/proto/ruto_v1"
)

type ApiKey struct {
	ruto_v1.UnsafeApiKeyServer
	usecase *usecase.Usecase
}

func NewApiKey(usecase *usecase.Usecase) *ApiKey {
	return &ApiKey{usecase: usecase}
}

func (h *ApiKey) List(ctx context.Context, _ *emptypb.Empty) (*ruto_v1.ApiKeyListRep, error) {
	items, err := h.usecase.List(ctx)
	if err != nil {
		return nil, err
	}
	return &ruto_v1.ApiKeyListRep{Results: lo.Map(items, dto.EncodeApiKeyMain)}, nil
}

func (h *ApiKey) Create(ctx context.Context, req *ruto_v1.ApiKeyCreateReq) (*ruto_v1.ApiKeyCreateRep, error) {
	id, key, err := h.usecase.Create(ctx, req.Name)
	if err != nil {
		return nil, err
	}
	return &ruto_v1.ApiKeyCreateRep{Id: id, Key: key}, nil
}

func (h *ApiKey) Delete(ctx context.Context, req *ruto_v1.ApiKeyDeleteReq) (*emptypb.Empty, error) {
	if err := h.usecase.Delete(ctx, req.Id); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}
