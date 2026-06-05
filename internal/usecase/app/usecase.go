package app

import (
	"context"
	"fmt"
	"strings"

	"github.com/rendau/ruto/internal/domain/app/model"
	commonModel "github.com/rendau/ruto/internal/domain/common/model"
	endpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
	rootModel "github.com/rendau/ruto/internal/domain/root/model"
	varsModel "github.com/rendau/ruto/internal/domain/vars/model"
	"github.com/rendau/ruto/internal/service/grpcreflect"

	"github.com/rendau/ruto/internal/errs"
)

type Usecase struct {
	svc         ServiceI
	rootSvc     RootServiceI
	endpointSvc EndpointServiceI
	swaggerSvc  SwaggerServiceI
	sessionSvc  SessionServiceI
}

func New(srv ServiceI, endpointSvc EndpointServiceI, swaggerSvc SwaggerServiceI, sessionSvc SessionServiceI, rootSvc ...RootServiceI) *Usecase {
	result := &Usecase{
		svc:         srv,
		endpointSvc: endpointSvc,
		swaggerSvc:  swaggerSvc,
		sessionSvc:  sessionSvc,
	}
	if len(rootSvc) > 0 {
		result.rootSvc = rootSvc[0]
	}
	return result
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

	err := u.validateEdit(ctx, obj, "")
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

func (u *Usecase) Interpolate(ctx context.Context, id string, variables varsModel.Vars) (*model.App, error) {
	return u.getInherited(ctx, id, variables, true)
}

func (u *Usecase) Inherited(ctx context.Context, id string, variables varsModel.Vars) (*model.App, error) {
	return u.getInherited(ctx, id, variables, false)
}

func (u *Usecase) getInherited(
	ctx context.Context,
	id string,
	variables varsModel.Vars,
	withInterpolate bool,
) (*model.App, error) {
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

	if u.rootSvc == nil {
		return nil, fmt.Errorf("rootSvc: nil")
	}
	rootObj, err := u.rootSvc.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("rootSvc.Get: %w", err)
	}
	if rootObj == nil {
		rootObj = rootModel.NewEmpty()
	}

	appObj.Variables = variables.Clone()
	rootObj.Apps = append(rootObj.Apps, appObj)
	rootObj.InheritDown()

	if withInterpolate {
		rootObj.Interpolate()
	}

	return appObj, nil
}

func (u *Usecase) Update(ctx context.Context, id string, obj *model.App) error {
	extractedSession := u.sessionSvc.FromContext(ctx)
	if extractedSession.Id == 0 {
		return errs.NotAuthorized
	}

	if id == "" {
		return errs.IdRequired
	}
	err := u.validateEdit(ctx, obj, id)
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
	filteredEndpoints := make([]*endpointModel.Endpoint, 0, len(endpoints))
	for _, endpoint := range endpoints {
		if endpoint == nil || endpoint.Type == endpointModel.TypeGRPC {
			continue
		}
		filteredEndpoints = append(filteredEndpoints, endpoint)
	}

	return buildSwaggerEndpointsDiff(swaggerEndpoints, filteredEndpoints), nil
}

func (u *Usecase) GetGrpcReflectionEndpoints(ctx context.Context, id string) ([]GrpcReflectionEndpoint, error) {
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

	if appObj.Backend.GrpcUrl == "" {
		return []GrpcReflectionEndpoint{}, nil
	}

	items, err := grpcreflect.LoadEndpoints(ctx, appObj.Backend.GrpcUrl)
	if err != nil {
		return nil, fmt.Errorf("grpc reflection: %w", err)
	}

	result := make([]GrpcReflectionEndpoint, 0, len(items))
	for _, item := range items {
		result = append(result, GrpcReflectionEndpoint{
			Service: item.Service,
			Method:  item.Method,
			Path:    item.Path,
		})
	}
	return result, nil
}

func (u *Usecase) validateEdit(ctx context.Context, obj *model.App, selfID string) error {
	if obj == nil {
		return fmt.Errorf("obj: nil")
	}

	if err := obj.Normalize(); err != nil {
		return fmt.Errorf("normalize: %w", err)
	}

	// ensure unique app name
	if err := u.ensureUniqueAppName(ctx, obj.Name, selfID); err != nil {
		return err
	}

	return nil
}

func (u *Usecase) ensureUniqueAppName(ctx context.Context, appName, selfID string) error {
	appName = strings.TrimSpace(appName)
	selfID = strings.TrimSpace(selfID)
	if appName == "" {
		return nil
	}

	listReq := &model.ListReq{
		ListParams: commonModel.ListParams{
			PageSize: 1,
		},
		NameEqCI: &appName,
	}
	if selfID != "" {
		listReq.ExcludeID = &selfID
	}

	items, _, err := u.svc.List(ctx, listReq)
	if err != nil {
		return fmt.Errorf("svc.List: %w", err)
	}
	if len(items) == 0 {
		return nil
	}

	return errs.ErrFull{
		Err:  errs.InvalidRequest,
		Desc: "app name must be unique",
		Fields: map[string]string{
			"name": "already exists",
		},
	}
}
