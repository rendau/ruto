package service

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/samber/lo"

	"github.com/rendau/ruto/internal/domain/clientstat/model"
)

type Service struct {
	repoDb RepoDbI
}

func New(repoDb RepoDbI) *Service { return &Service{repoDb: repoDb} }

// Add sums a report received at the given time into its bucket.
func (s *Service) Add(ctx context.Context, receivedAt time.Time, items []*model.ClientStat) error {
	if err := s.repoDb.Add(ctx, model.BucketOf(receivedAt), items); err != nil {
		return fmt.Errorf("repoDb.Add: %w", err)
	}
	return nil
}

func (s *Service) Overview(ctx context.Context, w *model.Window) (*model.Overview, error) {
	sums, err := s.repoDb.BucketSums(ctx, w.From)
	if err != nil {
		return nil, fmt.Errorf("repoDb.BucketSums: %w", err)
	}

	totals, err := s.repoDb.Totals(ctx, w.From)
	if err != nil {
		return nil, fmt.Errorf("repoDb.Totals: %w", err)
	}

	trustedSums, err := s.repoDb.TrustedBucketSums(ctx, w.From)
	if err != nil {
		return nil, fmt.Errorf("repoDb.TrustedBucketSums: %w", err)
	}

	return &model.Overview{
		Step:    w.Step,
		Points:  w.Fill(sums),
		Totals:  totals,
		Trusted: groupTrusted(w, trustedSums),
	}, nil
}

func (s *Service) Series(ctx context.Context, w *model.Window, ip string) (*model.Series, error) {
	buckets, err := s.repoDb.IpBuckets(ctx, ip, w.From)
	if err != nil {
		return nil, fmt.Errorf("repoDb.IpBuckets: %w", err)
	}
	return &model.Series{Step: w.Step, Points: w.Fill(buckets)}, nil
}

func (s *Service) Top(ctx context.Context, req *model.TopReq) ([]*model.Item, error) {
	items, err := s.repoDb.Top(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("repoDb.Top: %w", err)
	}
	return items, nil
}

// DeleteBefore removes the buckets older than the given time.
func (s *Service) DeleteBefore(ctx context.Context, before time.Time) (int64, error) {
	n, err := s.repoDb.DeleteBefore(ctx, before)
	if err != nil {
		return 0, fmt.Errorf("repoDb.DeleteBefore: %w", err)
	}
	return n, nil
}

// groupTrusted turns per-bucket sums of trusted clients into one timeline per
// name, busiest first.
func groupTrusted(w *model.Window, sums []*model.TrustedPoint) []*model.TrustedSum {
	byName := lo.GroupBy(sums, func(v *model.TrustedPoint) string { return v.Name })

	result := lo.MapToSlice(byName, func(name string, points []*model.TrustedPoint) *model.TrustedSum {
		return &model.TrustedSum{
			Name:     name,
			Requests: lo.SumBy(points, func(v *model.TrustedPoint) int64 { return v.Requests }),
			Points:   w.Fill(lo.Map(points, func(v *model.TrustedPoint, _ int) *model.Point { return &v.Point })),
		}
	})

	slices.SortFunc(result, func(a, b *model.TrustedSum) int {
		return cmp.Or(cmp.Compare(b.Requests, a.Requests), strings.Compare(a.Name, b.Name))
	})

	return result
}
