package root

import (
	"context"

	"github.com/rendau/ruto/internal/domain/root/model"
	sessionModel "github.com/rendau/ruto/internal/domain/session/model"
)

type ServiceI interface {
	Get(ctx context.Context) (*model.Root, error)
	Set(ctx context.Context, obj *model.Root) error
}

type SessionServiceI interface {
	FromContext(ctx context.Context) *sessionModel.Session
	CtxIsAuthorized(ctx context.Context) bool
	CtxIsAdmin(ctx context.Context) bool
}
