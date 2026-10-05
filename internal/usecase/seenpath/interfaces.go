package seenpath

import (
	"context"

	"github.com/rendau/ruto/internal/domain/seenpath/model"
)

type ServiceI interface {
	Add(ctx context.Context, items []*model.SeenPath) error
	ListByApp(ctx context.Context, appId string) ([]*model.SeenPath, error)
	DeleteByApp(ctx context.Context, appId string) error
}

type SessionServiceI interface {
	CtxIsAuthorized(ctx context.Context) bool
	CtxHasFullAppAccess(ctx context.Context) bool
	CtxGetAppIds(ctx context.Context) []string
}
