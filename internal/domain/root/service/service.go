package service

import (
	"context"
	"fmt"

	"github.com/rendau/ruto/internal/domain/root/model"
)

type Service struct {
	repoDb RepoDbI
}

func New(repoDb RepoDbI) *Service { return &Service{repoDb: repoDb} }

func (s *Service) Get(ctx context.Context) (*model.Root, error) {
	result, err := s.repoDb.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("repoDb.Get: %w", err)
	}
	if result == nil {
		return model.NewEmpty(), nil
	}

	return result, nil
}

func (s *Service) Set(ctx context.Context, obj *model.Root) error {
	apps := obj.Apps
	obj.Apps = nil
	defer func() {
		obj.Apps = apps
	}()

	if err := s.repoDb.Set(ctx, obj); err != nil {
		return fmt.Errorf("repoDb.Set: %w", err)
	}

	return nil
}
