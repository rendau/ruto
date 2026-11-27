package service

import (
	"context"
	"time"

	"github.com/rendau/ruto/internal/domain/seenorigin/model"
)

type RepoDbI interface {
	Add(ctx context.Context, day time.Time, items []*model.SeenOrigin) error
	List(ctx context.Context, since time.Time) ([]*model.SeenOrigin, error)
	DeleteItems(ctx context.Context, origins []string) error
	DeleteAll(ctx context.Context) error
	DeleteDaysBefore(ctx context.Context, day time.Time) (int64, error)
}
