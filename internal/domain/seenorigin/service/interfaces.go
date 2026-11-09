package service

import (
	"context"

	"github.com/rendau/ruto/internal/domain/seenorigin/model"
)

type RepoDbI interface {
	Add(ctx context.Context, items []*model.SeenOrigin) error
	List(ctx context.Context) ([]*model.SeenOrigin, error)
	DeleteItems(ctx context.Context, origins []string) error
	DeleteAll(ctx context.Context) error
}
