package service

import (
	"context"
	"time"

	"github.com/rendau/ruto/internal/domain/blockedclient/model"
)

type RepoDbI interface {
	List(ctx context.Context, pars *model.ListReq) ([]*model.BlockedClient, int64, error)
	Get(ctx context.Context, id string) (*model.BlockedClient, bool, error)
	Create(ctx context.Context, obj *model.BlockedClient) (string, error)
	Update(ctx context.Context, id string, obj *model.Edit) error
	Lock(ctx context.Context) error
	CloseExpired(ctx context.Context, ip string, now time.Time) error
	ActiveRules(ctx context.Context, now time.Time) ([]*model.Rule, error)
	Stats(ctx context.Context, now, agentsSince time.Time) (*model.Stats, error)
	RemoveMany(ctx context.Context, req *model.RemoveMany) (int64, error)
	Prune(ctx context.Context, endedBefore time.Time) (int64, error)
}
