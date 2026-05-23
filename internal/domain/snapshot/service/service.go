package service

import (
	"context"
	"fmt"

	"github.com/rendau/ruto/internal/domain/snapshot/model"
)

type Service struct {
	repoDb RepoDbI
}

func New(repoDb RepoDbI) *Service { return &Service{repoDb: repoDb} }

func (s *Service) GetVersion(ctx context.Context) (string, error) {
	result, err := s.repoDb.GetVersion(ctx)
	if err != nil {
		return "", fmt.Errorf("repoDb.GetVersion: %w", err)
	}
	return result, nil
}

func (s *Service) Get(ctx context.Context) (*model.Snapshot, error) {
	result, err := s.repoDb.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("repoDb.Get: %w", err)
	}
	if result == nil {
		return &model.Snapshot{
			Data: []byte("{}"),
		}, nil
	}
	if len(result.Data) == 0 {
		result.Data = []byte("{}")
	}

	return result, nil
}

func (s *Service) Set(ctx context.Context, obj *model.Snapshot) error {
	if len(obj.Data) == 0 {
		obj.Data = []byte("{}")
	}

	if err := s.repoDb.Set(ctx, obj); err != nil {
		return fmt.Errorf("repoDb.Set: %w", err)
	}

	return nil
}
