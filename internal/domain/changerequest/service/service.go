package service

import (
	"context"
	"fmt"
	"time"

	"github.com/rendau/ruto/internal/domain/changerequest/model"
	"github.com/rendau/ruto/internal/errs"
)

type Service struct {
	repoDb RepoDbI
}

func New(repoDb RepoDbI) *Service { return &Service{repoDb: repoDb} }

func (s *Service) List(ctx context.Context, pars *model.ListReq) ([]*model.ChangeRequest, int64, error) {
	items, tCount, err := s.repoDb.List(ctx, pars)
	if err != nil {
		return nil, 0, fmt.Errorf("repoDb.List: %w", err)
	}
	return items, tCount, nil
}

func (s *Service) Get(ctx context.Context, id int64, errNE bool) (*model.ChangeRequest, bool, error) {
	result, found, err := s.repoDb.Get(ctx, id)
	if err != nil {
		return nil, false, fmt.Errorf("repoDb.Get: %w", err)
	}
	if !found {
		if errNE {
			return nil, false, errs.ObjectNotFound
		}
		return nil, false, nil
	}
	return result, true, nil
}

// GetLocked loads the request under a row lock held until the ctx
// transaction ends.
func (s *Service) GetLocked(ctx context.Context, id int64) (*model.ChangeRequest, error) {
	if err := s.repoDb.Lock(ctx, id); err != nil {
		return nil, fmt.Errorf("repoDb.Lock: %w", err)
	}
	result, _, err := s.Get(ctx, id, true)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Service) Create(ctx context.Context, obj *model.ChangeRequest) (int64, error) {
	newId, err := s.repoDb.Create(ctx, obj)
	if err != nil {
		return 0, fmt.Errorf("repoDb.Create: %w", err)
	}
	return newId, nil
}

func (s *Service) Update(ctx context.Context, id int64, obj *model.Edit) error {
	if err := s.repoDb.Update(ctx, id, obj); err != nil {
		return fmt.Errorf("repoDb.Update: %w", err)
	}
	return nil
}

func (s *Service) PendingStats(ctx context.Context, stuckAfter time.Duration) (*model.PendingStats, error) {
	result, err := s.repoDb.PendingStats(ctx, time.Now().Add(-stuckAfter))
	if err != nil {
		return nil, fmt.Errorf("repoDb.PendingStats: %w", err)
	}
	return result, nil
}

// Cleanup expires stale pending requests and deletes old closed ones.
func (s *Service) Cleanup(ctx context.Context, pendingTTL, retention time.Duration) (expired, deleted int64, err error) {
	now := time.Now()

	expired, err = s.repoDb.ExpirePending(ctx, now.Add(-pendingTTL))
	if err != nil {
		return 0, 0, fmt.Errorf("repoDb.ExpirePending: %w", err)
	}

	deleted, err = s.repoDb.DeleteClosed(ctx, now.Add(-retention))
	if err != nil {
		return 0, 0, fmt.Errorf("repoDb.DeleteClosed: %w", err)
	}

	return expired, deleted, nil
}
