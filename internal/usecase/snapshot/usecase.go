package snapshot

import (
	"context"
	"fmt"

	"github.com/goccy/go-json"

	appModel "github.com/rendau/ruto/internal/domain/app/model"
	endpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
)

type Usecase struct {
	rootSvc     RootServiceI
	appSvc      AppServiceI
	endpointSvc EndpointServiceI
}

func New(rootSvc RootServiceI, appSvc AppServiceI, endpointSvc EndpointServiceI) *Usecase {
	return &Usecase{
		rootSvc:     rootSvc,
		appSvc:      appSvc,
		endpointSvc: endpointSvc,
	}
}

func (u *Usecase) Get(ctx context.Context) ([]byte, error) {
	rootObj, err := u.rootSvc.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("rootSvc.Get: %w", err)
	}

	apps, _, err := u.appSvc.List(ctx, &appModel.ListReq{})
	if err != nil {
		return nil, fmt.Errorf("appSvc.List: %w", err)
	}

	endpoints, _, err := u.endpointSvc.List(ctx, &endpointModel.ListReq{})
	if err != nil {
		return nil, fmt.Errorf("endpointSvc.List: %w", err)
	}

	appById := make(map[string]*appModel.App, len(apps))
	for _, app := range apps {
		app.Endpoints = make([]*endpointModel.Endpoint, 0)
		appById[app.Id] = app
	}

	for _, endpoint := range endpoints {
		app, ok := appById[endpoint.AppId]
		if !ok {
			continue
		}
		app.Endpoints = append(app.Endpoints, endpoint)
	}

	rootObj.Apps = apps

	result, err := json.Marshal(rootObj)
	if err != nil {
		return nil, fmt.Errorf("json.Marshal: %w", err)
	}

	return result, nil
}
