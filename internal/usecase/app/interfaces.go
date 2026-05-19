package app

import (
	"context"

	"github.com/rendau/ruto/internal/domain/app/model"
	sessionModel "github.com/rendau/ruto/internal/domain/session/model"
)

type ServiceI interface {
	List(ctx context.Context, pars *model.ListReq) ([]*model.App, int64, error)
	Get(ctx context.Context, id string, errNE bool) (*model.App, bool, error)
	Create(ctx context.Context, obj *model.App) (string, error)
	Update(ctx context.Context, id string, obj *model.App) error
	Delete(ctx context.Context, id string) error
}

type SessionServiceI interface {
	FromContext(ctx context.Context) *sessionModel.Session
}
