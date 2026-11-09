package seenorigin

import (
	"context"

	"github.com/rendau/ruto/internal/domain/seenorigin/model"
)

type ServiceI interface {
	Add(ctx context.Context, items []*model.SeenOrigin) error
	List(ctx context.Context) ([]*model.SeenOrigin, error)
	DeleteItems(ctx context.Context, origins []string) error
	DeleteAll(ctx context.Context) error
}

type SessionServiceI interface {
	CtxIsAuthorized(ctx context.Context) bool
	CtxIsAdmin(ctx context.Context) bool
}
