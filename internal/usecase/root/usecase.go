package root

import (
	"context"
	"fmt"

	domModel "github.com/rendau/ruto/internal/domain/root/model"
)

type Usecase struct {
	svc ServiceI
}

func New(srv ServiceI) *Usecase { return &Usecase{svc: srv} }

func (u *Usecase) Get(ctx context.Context) (*domModel.Main, error) {
	result, err := u.svc.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("svc.Get: %w", err)
	}
	return result, nil
}

func (u *Usecase) Set(ctx context.Context, obj *domModel.Edit) error {
	if err := u.svc.Set(ctx, obj); err != nil {
		return fmt.Errorf("svc.Update: %w", err)
	}
	return nil
}
