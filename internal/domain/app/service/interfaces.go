package service

import (
	"context"

	"github.com/rendau/ruto/internal/domain/app/model"
)

type RepoDbI interface {
	List(ctx context.Context, pars *model.ListReq) ([]*model.App, int64, error)
	Get(ctx context.Context, id string) (*model.App, bool, error)
	Create(ctx context.Context, obj *model.App) (string, error)
	Update(ctx context.Context, id string, obj *model.App) error
	Delete(ctx context.Context, id string) error
}
