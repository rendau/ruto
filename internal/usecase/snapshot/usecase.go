package snapshot

import (
	"context"
	"fmt"

	"github.com/goccy/go-json"
	"github.com/samber/lo"

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
	// fetch root
	rootObj, err := u.rootSvc.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("rootSvc.Get: %w", err)
	}

	// fetch apps
	apps, _, err := u.appSvc.List(ctx, &appModel.ListReq{})
	if err != nil {
		return nil, fmt.Errorf("appSvc.List: %w", err)
	}
	appById := lo.SliceToMap(apps, func(app *appModel.App) (string, *appModel.App) {
		app.Endpoints = make([]*endpointModel.Endpoint, 0, 20)
		return app.Id, app
	})

	// fetch endpoints
	endpoints, _, err := u.endpointSvc.List(ctx, &endpointModel.ListReq{})
	if err != nil {
		return nil, fmt.Errorf("endpointSvc.List: %w", err)
	}

	// link endpoints to apps
	lo.ForEach(endpoints, func(ep *endpointModel.Endpoint, _ int) {
		if app, ok := appById[ep.AppId]; ok {
			app.Endpoints = append(app.Endpoints, ep)
		}
	})

	rootObj.Apps = apps

	result, err := json.Marshal(rootObj)
	if err != nil {
		return nil, fmt.Errorf("json.Marshal: %w", err)
	}

	return result, nil
}
