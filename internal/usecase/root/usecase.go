package root

import (
	"context"
	"fmt"

	"github.com/rendau/ruto/internal/domain/root/model"
	"github.com/rendau/ruto/internal/errs"
)

type Usecase struct {
	svc        ServiceI
	sessionSvc SessionServiceI
}

func New(srv ServiceI, sessionSvc SessionServiceI) *Usecase {
	return &Usecase{
		svc:        srv,
		sessionSvc: sessionSvc,
	}
}

func (u *Usecase) Get(ctx context.Context) (*model.Root, error) {
	extractedSession := u.sessionSvc.FromContext(ctx)
	if extractedSession.Id == 0 {
		return nil, errs.NotAuthorized
	}

	result, err := u.svc.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("svc.Get: %w", err)
	}
	return result, nil
}

func (u *Usecase) Set(ctx context.Context, obj *model.Root) error {
	extractedSession := u.sessionSvc.FromContext(ctx)
	if extractedSession.Id == 0 {
		return errs.NotAuthorized
	}

	if err := u.svc.Set(ctx, obj); err != nil {
		return fmt.Errorf("svc.Set: %w", err)
	}
	return nil
}
