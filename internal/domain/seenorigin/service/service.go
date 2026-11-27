package service

import (
	"context"
	"fmt"
	"time"

	"github.com/rendau/ruto/internal/domain/seenorigin/model"
)

type Service struct {
	repoDb RepoDbI
}

func New(repoDb RepoDbI) *Service { return &Service{repoDb: repoDb} }

// Add counts the hits of a gateway report into the day of now.
func (s *Service) Add(ctx context.Context, now time.Time, items []*model.SeenOrigin) error {
	if err := s.repoDb.Add(ctx, now, items); err != nil {
		return fmt.Errorf("repoDb.Add: %w", err)
	}
	return nil
}

// List gives the origins with their hits over the window ending today.
func (s *Service) List(ctx context.Context, now time.Time) ([]*model.SeenOrigin, error) {
	items, err := s.repoDb.List(ctx, model.WindowStart(now))
	if err != nil {
		return nil, fmt.Errorf("repoDb.List: %w", err)
	}
	return items, nil
}

func (s *Service) DeleteItems(ctx context.Context, origins []string) error {
	if err := s.repoDb.DeleteItems(ctx, origins); err != nil {
		return fmt.Errorf("repoDb.DeleteItems: %w", err)
	}
	return nil
}

func (s *Service) DeleteAll(ctx context.Context) error {
	if err := s.repoDb.DeleteAll(ctx); err != nil {
		return fmt.Errorf("repoDb.DeleteAll: %w", err)
	}
	return nil
}

// Prune deletes the days that left the window ending today.
func (s *Service) Prune(ctx context.Context, now time.Time) (int64, error) {
	n, err := s.repoDb.DeleteDaysBefore(ctx, model.WindowStart(now))
	if err != nil {
		return 0, fmt.Errorf("repoDb.DeleteDaysBefore: %w", err)
	}
	return n, nil
}
