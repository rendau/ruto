package service

import (
	"context"
	"fmt"

	domModel "github.com/rendau/ruto/internal/domain/endpoint/model"
)

type Service struct {
	repoDb RepoDbI
}

func New(repoDb RepoDbI) *Service { return &Service{repoDb: repoDb} }

func (s *Service) List(ctx context.Context, pars *domModel.ListReq) ([]*domModel.Main, int64, error) {
	result, totalCount, err := s.repoDb.List(ctx, pars)
	if err != nil {
		return nil, 0, fmt.Errorf("repoDb.List: %w", err)
	}
	return result, totalCount, nil
}

func (s *Service) Get(ctx context.Context, id string) (*domModel.Main, bool, error) {
	result, found, err := s.repoDb.Get(ctx, id)
	if err != nil {
		return nil, false, fmt.Errorf("repoDb.Get: %w", err)
	}
	return result, found, nil
}

func (s *Service) Create(ctx context.Context, obj *domModel.Edit) error {
	err := s.repoDb.Create(ctx, obj)
	if err != nil {
		return fmt.Errorf("repoDb.Create: %w", err)
	}
	return nil
}

func (s *Service) Update(ctx context.Context, id string, obj *domModel.Edit) error {
	if err := s.repoDb.Update(ctx, id, obj); err != nil {
		return fmt.Errorf("repoDb.Update: %w", err)
	}
	return nil
}

func (s *Service) Delete(ctx context.Context, id string) error {
	if err := s.repoDb.Delete(ctx, id); err != nil {
		return fmt.Errorf("repoDb.Delete: %w", err)
	}
	return nil
}
