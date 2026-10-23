package service

import (
	"context"

	"github.com/rendau/ruto/internal/domain/seenpath/model"
)

type RepoDbI interface {
	Add(ctx context.Context, items []*model.SeenPath) error
	ListByApp(ctx context.Context, appId string) ([]*model.SeenPath, error)
	DeleteItems(ctx context.Context, appId string, keys []*model.SeenPathKey) error
	DeleteByApp(ctx context.Context, appId string) error
}
