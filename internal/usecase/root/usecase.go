package root

import (
	"context"
	"fmt"

	"github.com/rendau/ruto/internal/domain/root/model"
)

type Usecase struct {
	svc ServiceI
}

func New(srv ServiceI) *Usecase { return &Usecase{svc: srv} }

func (u *Usecase) Get(ctx context.Context) (*model.Root, error) {
	result, err := u.svc.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("svc.Get: %w", err)
	}
	return result, nil
}

func (u *Usecase) Set(ctx context.Context, obj *model.Root) error {
	if err := u.svc.Set(ctx, obj); err != nil {
		return fmt.Errorf("svc.Set: %w", err)
	}
	return nil
}
