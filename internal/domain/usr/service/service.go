package service

import (
	"context"
	"fmt"

	"github.com/rendau/ruto/internal/domain/usr/model"
	"github.com/rendau/ruto/internal/errs"
)

type Service struct {
	repoDb RepoDbI
}

func New(repoDb RepoDbI) *Service { return &Service{repoDb: repoDb} }

func (s *Service) List(ctx context.Context, pars *model.ListReq) ([]*model.Usr, int64, error) {
	items, tCount, err := s.repoDb.List(ctx, pars)
	if err != nil {
		return nil, 0, fmt.Errorf("repoDb.List: %w", err)
	}

	return items, tCount, nil
}

func (s *Service) Get(ctx context.Context, id int64, errNE bool) (*model.Usr, bool, error) {
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

func (s *Service) GetByUsernamePassword(ctx context.Context, username, password string) (*model.Usr, bool, error) {
	result, found, err := s.repoDb.GetByUsernamePassword(ctx, username, password)
	if err != nil {
		return nil, false, fmt.Errorf("repoDb.GetByUsernamePassword: %w", err)
	}

	return result, found, nil
}

func (s *Service) Create(ctx context.Context, obj *model.Edit) (int64, error) {
	newId, err := s.repoDb.Create(ctx, obj)
	if err != nil {
		return 0, fmt.Errorf("repoDb.Create: %w", err)
	}

	return newId, nil
}

func (s *Service) Update(ctx context.Context, id int64, obj *model.Edit) error {
	err := s.repoDb.Update(ctx, id, obj)
	if err != nil {
		return fmt.Errorf("repoDb.Update: %w", err)
	}

	return nil
}

func (s *Service) Delete(ctx context.Context, id int64) error {
	err := s.repoDb.Delete(ctx, id)
	if err != nil {
		return fmt.Errorf("repoDb.Delete: %w", err)
	}

	return nil
}
