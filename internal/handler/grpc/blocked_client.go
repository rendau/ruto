package grpc

import (
	"context"
	"time"

	"github.com/samber/lo"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/rendau/ruto/internal/handler/grpc/dto"
	usecase "github.com/rendau/ruto/internal/usecase/blockedclient"
	"github.com/rendau/ruto/pkg/proto/ruto_v1"
)

type BlockedClient struct {
	ruto_v1.UnsafeBlockedClientServer
	usecase *usecase.Usecase
}

func NewBlockedClient(usecase *usecase.Usecase) *BlockedClient {
	return &BlockedClient{usecase: usecase}
}

func (h *BlockedClient) List(ctx context.Context, req *ruto_v1.BlockedClientListReq) (*ruto_v1.BlockedClientListRep, error) {
	items, err := h.usecase.List(ctx, req.WithHistory)
	if err != nil {
		return nil, err
	}
	return &ruto_v1.BlockedClientListRep{Results: lo.Map(items, dto.EncodeBlockedClientMain)}, nil
}

func (h *BlockedClient) Create(ctx context.Context, req *ruto_v1.BlockedClientCreateReq) (*ruto_v1.BlockedClientMain, error) {
	item, err := h.usecase.Create(ctx, &usecase.CreateReq{
		Ip:       req.Ip,
		Reason:   req.Reason,
		Duration: time.Duration(req.DurationSeconds) * time.Second,
	})
	if err != nil {
		return nil, err
	}
	return dto.EncodeBlockedClientMain(item, 0), nil
}

func (h *BlockedClient) Remove(ctx context.Context, req *ruto_v1.BlockedClientRemoveReq) (*emptypb.Empty, error) {
	if _, err := h.usecase.Remove(ctx, &usecase.RemoveReq{Id: req.Id, Reason: req.Reason}); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (h *BlockedClient) RemoveAgentBlocks(ctx context.Context, req *ruto_v1.BlockedClientRemoveAgentBlocksReq) (*ruto_v1.BlockedClientRemoveAgentBlocksRep, error) {
	n, err := h.usecase.RemoveAgentBlocks(ctx, req.ApiKeyId, req.Reason)
	if err != nil {
		return nil, err
	}
	return &ruto_v1.BlockedClientRemoveAgentBlocksRep{Removed: n}, nil
}

func (h *BlockedClient) GetVersion(ctx context.Context, _ *emptypb.Empty) (*ruto_v1.BlockedClientVersion, error) {
	version, err := h.usecase.GetVersion(ctx)
	if err != nil {
		return nil, err
	}
	return &ruto_v1.BlockedClientVersion{Version: version}, nil
}

func (h *BlockedClient) GetActive(ctx context.Context, _ *emptypb.Empty) (*ruto_v1.BlockedClientActiveRep, error) {
	rules, version, err := h.usecase.GetActive(ctx)
	if err != nil {
		return nil, err
	}
	return &ruto_v1.BlockedClientActiveRep{Version: version, Rules: lo.Map(rules, dto.EncodeBlockedClientRule)}, nil
}
