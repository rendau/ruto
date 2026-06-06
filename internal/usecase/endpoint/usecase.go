package endpoint

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	appModel "github.com/rendau/ruto/internal/domain/app/model"
	"github.com/rendau/ruto/internal/domain/endpoint/model"
	rootModel "github.com/rendau/ruto/internal/domain/root/model"
	varsModel "github.com/rendau/ruto/internal/domain/vars/model"
	"github.com/rendau/ruto/internal/errs"
)

type Usecase struct {
	svc        ServiceI
	rootSvc    RootServiceI
	appSvc     AppServiceI
	sessionSvc SessionServiceI
	httpClient *http.Client
}

func New(srv ServiceI, sessionSvc SessionServiceI, services ...any) *Usecase {
	result := &Usecase{
		svc:        srv,
		sessionSvc: sessionSvc,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				DialContext: (&net.Dialer{
					Timeout: 2 * time.Second,
				}).DialContext,
				TLSHandshakeTimeout: 2 * time.Second,
				TLSClientConfig:     &tls.Config{InsecureSkipVerify: true},
				MaxIdleConnsPerHost: 20,
			},
		},
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
	if !u.sessionSvc.CtxIsAuthorized(ctx) {
		return nil, 0, errs.NotAuthorized
	}

	items, tCount, err := u.svc.List(ctx, pars)
	if err != nil {
		return nil, 0, fmt.Errorf("svc.List: %w", err)
	}

	return items, tCount, err
}

func (u *Usecase) Create(ctx context.Context, obj *model.Endpoint) (string, error) {
	if !u.sessionSvc.CtxIsAuthorized(ctx) {
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
	if !u.sessionSvc.CtxIsAuthorized(ctx) {
		return nil, errs.NotAuthorized
	}

	result, _, err := u.svc.Get(ctx, id, true)
	if err != nil {
		return nil, fmt.Errorf("svc.Get: %w", err)
	}

	return result, nil
}

func (u *Usecase) Interpolate(ctx context.Context, id string, variables varsModel.Vars) (*model.Endpoint, error) {
	return u.getInherited(ctx, id, variables, true)
}

func (u *Usecase) Inherited(ctx context.Context, id string, variables varsModel.Vars) (*model.Endpoint, error) {
	return u.getInherited(ctx, id, variables, false)
}

func (u *Usecase) Update(ctx context.Context, id string, obj *model.Endpoint) error {
	if !u.sessionSvc.CtxIsAuthorized(ctx) {
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
	if !u.sessionSvc.CtxIsAuthorized(ctx) {
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

func (u *Usecase) getInherited(
	ctx context.Context,
	id string,
	variables varsModel.Vars,
	withInterpolate bool,
) (*model.Endpoint, error) {
	if !u.sessionSvc.CtxIsAuthorized(ctx) {
		return nil, errs.NotAuthorized
	}

	epObj, _, err := u.resolveInherited(ctx, id, variables, withInterpolate)
	if err != nil {
		return nil, err
	}

	return epObj, nil
}

// resolveInherited loads the endpoint together with its parent app and root,
// applies the Root→App→Endpoint inheritance (and optionally interpolation), and
// returns the resolved endpoint and its resolved app. It performs no
// authorization check — callers must do that themselves.
func (u *Usecase) resolveInherited(
	ctx context.Context,
	id string,
	variables varsModel.Vars,
	withInterpolate bool,
) (*model.Endpoint, *appModel.App, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, nil, errs.IdRequired
	}

	epObj, _, err := u.svc.Get(ctx, id, true)
	if err != nil {
		return nil, nil, fmt.Errorf("svc.Get: %w", err)
	}
	if epObj == nil {
		return nil, nil, fmt.Errorf("svc.Get: nil endpoint")
	}

	appObj, _, err := u.appSvc.Get(ctx, epObj.AppId, true)
	if err != nil {
		return nil, nil, fmt.Errorf("appSvc.Get: %w", err)
	}
	if appObj == nil {
		return nil, nil, fmt.Errorf("appSvc.Get: nil app")
	}

	rootObj, err := u.rootSvc.Get(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("rootSvc.Get: %w", err)
	}
	if rootObj == nil {
		rootObj = rootModel.NewEmpty()
	}

	epObj.Variables = variables.Clone()

	appObj.Endpoints = []*model.Endpoint{epObj}
	rootObj.Apps = []*appModel.App{appObj}
	rootObj.InheritDown()
	if withInterpolate {
		rootObj.Interpolate()
	}

	return epObj, appObj, nil
}
