package seenorigin

import (
	"context"
	"time"

	"github.com/rendau/ruto/internal/domain/seenorigin/model"
)

type ServiceI interface {
	Add(ctx context.Context, now time.Time, items []*model.SeenOrigin) error
	List(ctx context.Context, now time.Time) ([]*model.SeenOrigin, error)
	DeleteItems(ctx context.Context, origins []string) error
	DeleteAll(ctx context.Context) error
	Prune(ctx context.Context, now time.Time) (int64, error)
}

type SessionServiceI interface {
	CtxIsAuthorized(ctx context.Context) bool
	CtxIsAdmin(ctx context.Context) bool
}
