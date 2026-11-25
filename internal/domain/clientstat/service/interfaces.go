package service

import (
	"context"
	"time"

	"github.com/rendau/ruto/internal/domain/clientstat/model"
)

type RepoDbI interface {
	Add(ctx context.Context, bucket time.Time, items []*model.ClientStat) error
	BucketSums(ctx context.Context, since time.Time) ([]*model.Point, error)
	IpBuckets(ctx context.Context, ip string, since time.Time) ([]*model.Point, error)
	TrustedBucketSums(ctx context.Context, since time.Time) ([]*model.TrustedPoint, error)
	Totals(ctx context.Context, since time.Time) (*model.Totals, error)
	Top(ctx context.Context, req *model.TopReq) ([]*model.Item, error)
	DeleteBefore(ctx context.Context, before time.Time) (int64, error)
}
