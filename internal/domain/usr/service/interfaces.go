package service

import (
	"context"

	"github.com/rendau/ruto/internal/domain/usr/model"
)

type RepoDbI interface {
	List(ctx context.Context, pars *model.ListReq) ([]*model.Usr, int64, error)
	Get(ctx context.Context, id int64) (*model.Usr, bool, error)
	GetByUsernamePassword(ctx context.Context, username, password string) (*model.Usr, bool, error)
	Create(ctx context.Context, obj *model.Edit) (int64, error)
	Update(ctx context.Context, id int64, obj *model.Edit) error
	Delete(ctx context.Context, id int64) error
}
