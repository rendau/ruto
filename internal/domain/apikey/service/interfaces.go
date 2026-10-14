package service

import (
	"context"

	"github.com/rendau/ruto/internal/domain/apikey/model"
)

type RepoDbI interface {
	List(ctx context.Context, pars *model.ListReq) ([]*model.ApiKey, int64, error)
	Get(ctx context.Context, id string) (*model.ApiKey, bool, error)
	Create(ctx context.Context, obj *model.ApiKey) (string, error)
	Delete(ctx context.Context, id string) error
	TouchLastUsed(ctx context.Context, id string) error
}
