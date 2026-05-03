package service

import (
	"context"
	"fmt"

	domModel "github.com/rendau/ruto/internal/domain/root/model"
)

type Service struct {
	repoDb RepoDbI
}

func New(repoDb RepoDbI) *Service { return &Service{repoDb: repoDb} }

func (s *Service) Get(ctx context.Context) (*domModel.Main, error) {
	result, err := s.repoDb.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("repoDb.Get: %w", err)
	}
	if result == nil {
		return domModel.NewEmpty(), nil
	}
	return result, nil
}

func (s *Service) Set(ctx context.Context, obj *domModel.Edit) error {
	if err := s.repoDb.Set(ctx, obj); err != nil {
		return fmt.Errorf("repoDb.Update: %w", err)
	}
	return nil
}
