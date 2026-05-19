package usr

import (
	"context"
	"fmt"
	"strings"

	"github.com/rendau/ruto/internal/domain/usr/model"
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

func (u *Usecase) Login(ctx context.Context, username, password string) (string, error) {
	username = strings.TrimSpace(username)

	item, found, err := u.svc.GetByUsernamePassword(ctx, username, password)
	if err != nil {
		return "", fmt.Errorf("svc.GetByUsernamePassword: %w", err)
	}
	if !found {
		return "", errs.NotAuthorized
	}

	token, err := u.sessionSvc.CreateToken(item.Id)
	if err != nil {
		return "", fmt.Errorf("sessionSvc.CreateToken: %w", err)
	}

	return token, nil
}

func (u *Usecase) GetProfile(ctx context.Context) (*model.Usr, error) {
	extractedSession := u.sessionSvc.FromContext(ctx)
	if extractedSession.Id == 0 {
		return nil, errs.NotAuthorized
	}

	item, found, err := u.svc.Get(ctx, extractedSession.Id, false)
	if err != nil {
		return nil, fmt.Errorf("svc.Get: %w", err)
	}
	if !found {
		return nil, errs.NotAuthorized
	}

	item.Password = ""

	return item, nil
}

func (u *Usecase) List(ctx context.Context, pars *model.ListReq) ([]*model.Usr, int64, error) {
	if u.sessionSvc.FromContext(ctx).Id == 0 {
		return nil, 0, errs.NotAuthorized
	}

	items, tCount, err := u.svc.List(ctx, pars)
	if err != nil {
		return nil, 0, fmt.Errorf("svc.List: %w", err)
	}

	return items, tCount, nil
}

func (u *Usecase) Get(ctx context.Context, id int64) (*model.Usr, error) {
	if u.sessionSvc.FromContext(ctx).Id == 0 {
		return nil, errs.NotAuthorized
	}

	if id == 0 {
		return nil, errs.IdRequired
	}

	item, _, err := u.svc.Get(ctx, id, true)
	if err != nil {
		return nil, fmt.Errorf("svc.Get: %w", err)
	}

	item.Password = ""

	return item, nil
}

func (u *Usecase) Create(ctx context.Context, obj *model.Usr) (int64, error) {
	if u.sessionSvc.FromContext(ctx).Id == 0 {
		return 0, errs.NotAuthorized
	}

	if err := u.validateEdit(obj); err != nil {
		return 0, err
	}

	newId, err := u.svc.Create(ctx, obj)
	if err != nil {
		return 0, fmt.Errorf("svc.Create: %w", err)
	}
	return newId, nil
}

func (u *Usecase) Update(ctx context.Context, id int64, obj *model.Usr) error {
	if u.sessionSvc.FromContext(ctx).Id == 0 {
		return errs.NotAuthorized
	}

	if id == 0 {
		return errs.IdRequired
	}
	if err := u.validateEdit(obj); err != nil {
		return err
	}

	if err := u.svc.Update(ctx, id, obj); err != nil {
		return fmt.Errorf("svc.Update: %w", err)
	}
	return nil
}

func (u *Usecase) Delete(ctx context.Context, id int64) error {
	if u.sessionSvc.FromContext(ctx).Id == 0 {
		return errs.NotAuthorized
	}

	if id == 0 {
		return errs.IdRequired
	}

	if err := u.svc.Delete(ctx, id); err != nil {
		return fmt.Errorf("svc.Delete: %w", err)
	}
	return nil
}

func (u *Usecase) validateEdit(obj *model.Usr) error {
	if obj == nil {
		return errs.InvalidRequest
	}
	if err := obj.Normalize(); err != nil {
		return fmt.Errorf("normalize: %w", err)
	}
	return nil
}
