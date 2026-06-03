package endpoint

import (
	"context"
	"fmt"
	"strings"

	appModel "github.com/rendau/ruto/internal/domain/app/model"
	"github.com/rendau/ruto/internal/domain/endpoint/model"
	variableModel "github.com/rendau/ruto/internal/domain/variable/model"
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

func (u *Usecase) GetVariablesEffective(ctx context.Context, id string, appID string, variables []variableModel.Variable) ([]variableModel.Variable, error) {
	extractedSession := u.sessionSvc.FromContext(ctx)
	if extractedSession.Id == 0 {
		return nil, errs.NotAuthorized
	}

	if u.rootSvc == nil {
		return nil, fmt.Errorf("rootSvc: nil")
	}
	if u.appSvc == nil {
		return nil, fmt.Errorf("appSvc: nil")
	}

	rootObj, err := u.rootSvc.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("rootSvc.Get: %w", err)
	}
	variables, err = variableModel.NormalizeList(variables)
	if err != nil {
		return nil, fmt.Errorf("variables: %w", err)
	}

	endpointObj := &model.Endpoint{
		AppId:     strings.TrimSpace(appID),
		Variables: variables,
	}
	id = strings.TrimSpace(id)
	if id != "" {
		endpointObj, _, err = u.svc.Get(ctx, id, true)
		if err != nil {
			return nil, fmt.Errorf("svc.Get: %w", err)
		}
		endpointObj.Variables = variables
	}
	if endpointObj.AppId == "" {
		return nil, fmt.Errorf("app_id: empty")
	}

	appObj, _, err := u.appSvc.Get(ctx, endpointObj.AppId, true)
	if err != nil {
		return nil, fmt.Errorf("appSvc.Get: %w", err)
	}
	if appObj == nil {
		appObj = &appModel.App{}
	}

	effective, err := rootObj.EffectiveVariables(appObj, endpointObj)
	if err != nil {
		return nil, fmt.Errorf("variables: %w", err)
	}
	result, err := variableModel.ResolveList(effective)
	if err != nil {
		return nil, fmt.Errorf("variables: %w", err)
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
