package service

import (
	"context"

	"github.com/rendau/ruto/internal/domain/snapshot/model"
)

type RepoDbI interface {
	GetVersion(ctx context.Context) (string, error)
	Get(ctx context.Context) (*model.Snapshot, error)
	Set(ctx context.Context, obj *model.Snapshot) error
}
