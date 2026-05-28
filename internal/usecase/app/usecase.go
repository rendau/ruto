package app

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/rendau/ruto/internal/domain/app/model"
	endpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
	swaggerService "github.com/rendau/ruto/internal/service/swagger"

	"github.com/rendau/ruto/internal/errs"
)

type Usecase struct {
	svc         ServiceI
	endpointSvc EndpointServiceI
	swaggerSvc  SwaggerServiceI
	sessionSvc  SessionServiceI
}

func New(srv ServiceI, endpointSvc EndpointServiceI, swaggerSvc SwaggerServiceI, sessionSvc SessionServiceI) *Usecase {
	return &Usecase{
		svc:         srv,
		endpointSvc: endpointSvc,
		swaggerSvc:  swaggerSvc,
		sessionSvc:  sessionSvc,
	}
}

func (u *Usecase) List(ctx context.Context, pars *model.ListReq) ([]*model.App, int64, error) {
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

func (u *Usecase) Create(ctx context.Context, obj *model.App) (string, error) {
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

func (u *Usecase) Get(ctx context.Context, id string) (*model.App, error) {
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

func (u *Usecase) Update(ctx context.Context, id string, obj *model.App) error {
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

func (u *Usecase) GetSwaggerEndpointsDiff(ctx context.Context, id string) (*SwaggerEndpointsDiff, error) {
	extractedSession := u.sessionSvc.FromContext(ctx)
	if extractedSession.Id == 0 {
		return nil, errs.NotAuthorized
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, errs.IdRequired
	}

	appObj, _, err := u.svc.Get(ctx, id, true)
	if err != nil {
		return nil, fmt.Errorf("svc.Get: %w", err)
	}

	swaggerURL := strings.TrimSpace(appObj.Backend.SwaggerUrl)
	if swaggerURL == "" {
		return &SwaggerEndpointsDiff{}, nil
	}
	if u.swaggerSvc == nil {
		return nil, fmt.Errorf("swaggerSvc: nil")
	}

	swaggerEndpoints, err := u.swaggerSvc.LoadEndpoints(ctx, swaggerURL)
	if err != nil {
		return nil, fmt.Errorf("swaggerSvc.LoadEndpoints: %w", err)
	}

	endpoints, _, err := u.endpointSvc.List(ctx, &endpointModel.ListReq{
		AppId: &id,
	})
	if err != nil {
		return nil, fmt.Errorf("endpointSvc.List: %w", err)
	}

	return buildSwaggerEndpointsDiff(swaggerEndpoints, endpoints), nil
}

func (u *Usecase) validateEdit(obj *model.App, forCreate bool) error {
	if obj == nil {
		return fmt.Errorf("obj: nil")
	}
	if err := obj.Normalize(); err != nil {
		return fmt.Errorf("normalize: %w", err)
	}
	return nil
}

func buildSwaggerEndpointsDiff(swaggerEndpoints []swaggerService.Endpoint, registeredEndpoints []*endpointModel.Endpoint) *SwaggerEndpointsDiff {
	swaggerSet := make(map[string]SwaggerEndpoint, len(swaggerEndpoints))
	for _, item := range swaggerEndpoints {
		swaggerSet[swaggerEndpointKey(item.Method, item.Path)] = SwaggerEndpoint{
			Method: item.Method,
			Path:   item.Path,
		}
	}

	registeredSet := make(map[string]SwaggerEndpoint, len(registeredEndpoints))
	for _, item := range registeredEndpoints {
		method := normalizeComparableMethod(item.Method)
		path := normalizeComparablePath(item.Path)
		if method == "" || path == "" {
			continue
		}
		registeredSet[swaggerEndpointKey(method, path)] = SwaggerEndpoint{
			Method: method,
			Path:   path,
		}
	}

	unregistered := make([]SwaggerEndpoint, 0)
	for key, item := range swaggerSet {
		if _, ok := registeredSet[key]; ok {
			continue
		}
		unregistered = append(unregistered, item)
	}
	slices.SortFunc(unregistered, func(a, b SwaggerEndpoint) int {
		if cmp := strings.Compare(a.Path, b.Path); cmp != 0 {
			return cmp
		}
		return strings.Compare(a.Method, b.Method)
	})

	registeredInvalid := make([]SwaggerEndpoint, 0)
	for key, item := range registeredSet {
		if _, ok := swaggerSet[key]; ok {
			continue
		}
		registeredInvalid = append(registeredInvalid, item)
	}
	slices.SortFunc(registeredInvalid, func(a, b SwaggerEndpoint) int {
		if cmp := strings.Compare(a.Path, b.Path); cmp != 0 {
			return cmp
		}
		return strings.Compare(a.Method, b.Method)
	})

	return &SwaggerEndpointsDiff{
		Unregistered:      unregistered,
		RegisteredInvalid: registeredInvalid,
	}
}

func swaggerEndpointKey(method, path string) string {
	return method + " " + path
}

func normalizeComparableMethod(method string) string {
	return strings.ToUpper(strings.TrimSpace(method))
}

func normalizeComparablePath(path string) string {
	p := strings.TrimSpace(path)
	p = strings.Trim(p, "/")
	if p == "" {
		return "/"
	}
	return "/" + p
}
