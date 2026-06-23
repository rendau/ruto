package endpoint

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/samber/lo"

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

	// Any authorized user may view all endpoints; secrets of endpoints whose app
	// the user can't manage are masked.
	items = lo.Map(items, func(item *model.Endpoint, _ int) *model.Endpoint {
		if u.canManage(ctx, item.AppId) {
			return item
		}
		return item.Redacted()
	})

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
	if err = u.requireAppAccess(ctx, obj.AppId); err != nil {
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

	if result != nil && !u.canManage(ctx, result.AppId) {
		result = result.Redacted()
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

	current, _, err := u.svc.Get(ctx, id, true)
	if err != nil {
		return fmt.Errorf("svc.Get: %w", err)
	}
	if err = u.requireAppAccess(ctx, current.AppId); err != nil {
		return err
	}

	err = u.validateEdit(obj, false)
	if err != nil {
		return err
	}
	// Moving an endpoint to a different app requires access to the target app too.
	if obj.AppId != current.AppId {
		if err = u.requireAppAccess(ctx, obj.AppId); err != nil {
			return err
		}
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

	current, _, err := u.svc.Get(ctx, id, true)
	if err != nil {
		return fmt.Errorf("svc.Get: %w", err)
	}
	if err = u.requireAppAccess(ctx, current.AppId); err != nil {
		return err
	}

	err = u.svc.Delete(ctx, id)
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

	if !u.canManage(ctx, epObj.AppId) {
		epObj = epObj.Redacted()
	}

	return epObj, nil
}

func (u *Usecase) requireAppAccess(ctx context.Context, appId string) error {
	if u.canManage(ctx, appId) {
		return nil
	}
	return errs.NoPermission
}

// canManage reports whether the current session may modify endpoints of the app
// (and see their secrets): full app access (admin/all_apps) or the app is in the
// user's app_ids.
func (u *Usecase) canManage(ctx context.Context, appId string) bool {
	if u.sessionSvc.CtxHasFullAppAccess(ctx) {
		return true
	}
	return lo.Contains(u.sessionSvc.CtxGetAppIds(ctx), appId)
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
