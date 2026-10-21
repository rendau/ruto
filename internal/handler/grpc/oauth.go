package grpc

import (
	"context"

	"github.com/samber/lo"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/rendau/ruto/internal/handler/grpc/dto"
	usecase "github.com/rendau/ruto/internal/usecase/oauth"
	"github.com/rendau/ruto/pkg/proto/ruto_v1"
)

type OAuth struct {
	ruto_v1.UnsafeOAuthServer
	usecase *usecase.Usecase
}

func NewOAuth(usecase *usecase.Usecase) *OAuth {
	return &OAuth{usecase: usecase}
}

func (h *OAuth) DescribeAuthorization(ctx context.Context, req *ruto_v1.OAuthAuthorizeReq) (*ruto_v1.OAuthAuthorizeInfoRep, error) {
	info, err := h.usecase.DescribeAuthorization(ctx, dto.DecodeOAuthAuthorizeReq(req))
	if err != nil {
		return nil, err
	}
	return dto.EncodeOAuthAuthorizeInfo(info), nil
}

func (h *OAuth) Authorize(ctx context.Context, req *ruto_v1.OAuthAuthorizeReq) (*ruto_v1.OAuthAuthorizeRep, error) {
	redirectUrl, err := h.usecase.Authorize(ctx, dto.DecodeOAuthAuthorizeReq(req), req.Approve)
	if err != nil {
		return nil, err
	}
	return &ruto_v1.OAuthAuthorizeRep{RedirectUrl: redirectUrl}, nil
}

func (h *OAuth) ListGrants(ctx context.Context, _ *emptypb.Empty) (*ruto_v1.OAuthGrantListRep, error) {
	items, err := h.usecase.ListGrants(ctx)
	if err != nil {
		return nil, err
	}
	return &ruto_v1.OAuthGrantListRep{Results: lo.Map(items, dto.EncodeOAuthGrantMain)}, nil
}

func (h *OAuth) DeleteGrant(ctx context.Context, req *ruto_v1.OAuthGrantDeleteReq) (*emptypb.Empty, error) {
	if err := h.usecase.DeleteGrant(ctx, req.Id); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}
