package endpoint

import (
	"context"
	"fmt"
	"strings"

	"github.com/rendau/ruto/internal/domain/endpoint/model"
	"github.com/rendau/ruto/internal/errs"
)

type Usecase struct {
	svc        ServiceI
	rootSvc    RootServiceI
	appSvc     AppServiceI
	sessionSvc SessionServiceI
}

func New(srv ServiceI, sessionSvc SessionServiceI, services ...any) *Usecase {
	result := &Usecase{
		svc:        srv,
		sessionSvc: sessionSvc,
	}
	for _, service := range services {
		if rootSvc, ok := service.(RootServiceI); ok {
			result.rootSvc = rootSvc
		}
		if appSvc, ok := service.(AppServiceI); ok {
			result.appSvc = appSvc
		}
	}
	return result
}

func (u *Usecase) List(ctx context.Context, pars *model.ListReq) ([]*model.Endpoint, int64, error) {
	extractedSession := u.sessionSvc.FromContext(ctx)
	if extractedSession.Id == 0 {
		return nil, 0, errs.NotAuthorized
	}

	items, tCount, err := u.svc.List(ctx, pars)
	if err != nil {
		return nil, 0, fmt.Errorf("svc.List: %w", err)
	}

	return items, tCount, err
}

func (u *Usecase) Create(ctx context.Context, obj *model.Endpoint) (string, error) {
	extractedSession := u.sessionSvc.FromContext(ctx)
	if extractedSession.Id == 0 {
		return "", errs.NotAuthorized
	}

	err := u.validateEdit(obj, true)
	if err != nil {
		return "", err
	}

	newId, err := u.svc.Create(ctx, obj)
	if err != nil {
		return "", fmt.Errorf("svc.Create: %w", err)
	}

	return newId, nil
}

func (u *Usecase) Get(ctx context.Context, id string) (*model.Endpoint, error) {
	extractedSession := u.sessionSvc.FromContext(ctx)
	if extractedSession.Id == 0 {
		return nil, errs.NotAuthorized
	}

	result, _, err := u.svc.Get(ctx, id, true)
	if err != nil {
		return nil, fmt.Errorf("svc.Get: %w", err)
	}

	return result, nil
}

func (u *Usecase) Update(ctx context.Context, id string, obj *model.Endpoint) error {
	extractedSession := u.sessionSvc.FromContext(ctx)
	if extractedSession.Id == 0 {
		return errs.NotAuthorized
	}

	if id == "" {
		return errs.IdRequired
	}
	err := u.validateEdit(obj, false)
	if err != nil {
		return err
	}

	err = u.svc.Update(ctx, id, obj)
	if err != nil {
		return fmt.Errorf("svc.Update: %w", err)
	}

	return nil
}

func (u *Usecase) Delete(ctx context.Context, id string) error {
	extractedSession := u.sessionSvc.FromContext(ctx)
	if extractedSession.Id == 0 {
		return errs.NotAuthorized
	}

	if id == "" {
		return errs.IdRequired
	}
	err := u.svc.Delete(ctx, id)
	if err != nil {
		return fmt.Errorf("svc.Delete: %w", err)
	}

	return nil
}

func (u *Usecase) validateEdit(obj *model.Endpoint, forCreate bool) error {
	obj.AppId = strings.TrimSpace(obj.AppId)
	if obj.AppId == "" {
		return fmt.Errorf("app_id: empty")
	}

	if err := obj.Normalize(); err != nil {
		return fmt.Errorf("normalize: %w", err)
	}

	return nil
}
