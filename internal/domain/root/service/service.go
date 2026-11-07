package service

import (
	"context"
	"fmt"

	compressionModel "github.com/rendau/ruto/internal/domain/compression/model"
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
	// A root stored before a section got defaults has it empty: every reader
	// (admin, MCP, the effective endpoint) sees the values the gateway applies.
	result.Compression = compressionModel.Merge(compressionModel.Default(), result.Compression)

	return result, nil
}

func (s *Service) Set(ctx context.Context, obj *model.Root) error {
	apps := obj.Apps
	obj.Apps = nil
	defer func() {
		obj.Apps = apps
	}()

	// Normalize here, not only when the snapshot is built: a bad entry must be
	// refused at save time, not surface later as a failed deploy.
	if err := obj.Normalize(); err != nil {
		return fmt.Errorf("normalize: %w", err)
	}

	if err := s.repoDb.Set(ctx, obj); err != nil {
		return fmt.Errorf("repoDb.Set: %w", err)
	}

	return nil
}
