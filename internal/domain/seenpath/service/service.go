package service

import (
	"context"
	"fmt"

	"github.com/rendau/ruto/internal/domain/seenpath/model"
)

type Service struct {
	repoDb RepoDbI
}

func New(repoDb RepoDbI) *Service { return &Service{repoDb: repoDb} }

func (s *Service) Add(ctx context.Context, items []*model.SeenPath) error {
	if err := s.repoDb.Add(ctx, items); err != nil {
		return fmt.Errorf("repoDb.Add: %w", err)
	}
	return nil
}

func (s *Service) ListByApp(ctx context.Context, appId string) ([]*model.SeenPath, error) {
	items, err := s.repoDb.ListByApp(ctx, appId)
	if err != nil {
		return nil, fmt.Errorf("repoDb.ListByApp: %w", err)
	}
	return items, nil
}

func (s *Service) DeleteByApp(ctx context.Context, appId string) error {
	if err := s.repoDb.DeleteByApp(ctx, appId); err != nil {
		return fmt.Errorf("repoDb.DeleteByApp: %w", err)
	}
	return nil
}
