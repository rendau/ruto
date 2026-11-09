package service

import (
	"context"
	"fmt"

	"github.com/rendau/ruto/internal/domain/seenorigin/model"
)

type Service struct {
	repoDb RepoDbI
}

func New(repoDb RepoDbI) *Service { return &Service{repoDb: repoDb} }

func (s *Service) Add(ctx context.Context, items []*model.SeenOrigin) error {
	if err := s.repoDb.Add(ctx, items); err != nil {
		return fmt.Errorf("repoDb.Add: %w", err)
	}
	return nil
}

func (s *Service) List(ctx context.Context) ([]*model.SeenOrigin, error) {
	items, err := s.repoDb.List(ctx)
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
