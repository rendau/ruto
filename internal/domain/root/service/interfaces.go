package service

import (
	"context"

	domModel "github.com/rendau/ruto/internal/domain/root/model"
)

type RepoDbI interface {
	Get(ctx context.Context) (*domModel.Main, error)
	Set(ctx context.Context, obj *domModel.Edit) error
}
