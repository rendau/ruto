package service

import (
	"context"

	domModel "github.com/rendau/ruto/internal/domain/app/model"
)

type RepoDbI interface {
	List(ctx context.Context, pars *domModel.ListReq) ([]*domModel.Main, int64, error)
	Get(ctx context.Context, id string) (*domModel.Main, bool, error)
	Create(ctx context.Context, obj *domModel.Edit) error
	Update(ctx context.Context, id string, obj *domModel.Edit) error
	Delete(ctx context.Context, id string) error
}
