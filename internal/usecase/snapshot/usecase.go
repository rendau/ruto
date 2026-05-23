package snapshot

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/goccy/go-json"
	"github.com/samber/lo"

	appModel "github.com/rendau/ruto/internal/domain/app/model"
	endpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
	rootModel "github.com/rendau/ruto/internal/domain/root/model"
	snapshotModel "github.com/rendau/ruto/internal/domain/snapshot/model"
)

type Usecase struct {
	snapshotSvc ServiceI
	rootSvc     RootServiceI
	appSvc      AppServiceI
	endpointSvc EndpointServiceI
}

func New(
	snapshotSvc ServiceI,
	rootSvc RootServiceI,
	appSvc AppServiceI,
	endpointSvc EndpointServiceI,
) *Usecase {
	return &Usecase{
		snapshotSvc: snapshotSvc,
		rootSvc:     rootSvc,
		appSvc:      appSvc,
		endpointSvc: endpointSvc,
	}
}

func (u *Usecase) GetVersion(ctx context.Context) (string, error) {
	version, err := u.snapshotSvc.GetVersion(ctx)
	if err != nil {
		return "", fmt.Errorf("snapshotSvc.GetVersion: %w", err)
	}
	return version, nil
}

func (u *Usecase) Get(ctx context.Context) ([]byte, error) {
	result, err := u.snapshotSvc.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("snapshotSvc.Get: %w", err)
	}
	return result.Data, nil
}

func (u *Usecase) Refresh(ctx context.Context) error {
	rootObj, err := u.generateFromDb(ctx)
	if err != nil {
		return fmt.Errorf("generateFromDb: %w", err)
	}

	result, err := json.Marshal(rootObj)
	if err != nil {
		return fmt.Errorf("json.Marshal: %w", err)
	}

	sum := sha256.Sum256(result)
	newVersion := hex.EncodeToString(sum[:])

	err = u.snapshotSvc.Set(ctx, &snapshotModel.Snapshot{
		Hash: newVersion,
		Data: result,
	})
	if err != nil {
		return fmt.Errorf("snapshotSvc.Set: %w", err)
	}

	return nil
}

func (u *Usecase) generateFromDb(ctx context.Context) (*rootModel.Root, error) {
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
	appByID := lo.SliceToMap(apps, func(app *appModel.App) (string, *appModel.App) {
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
		if app, ok := appByID[ep.AppId]; ok {
			app.Endpoints = append(app.Endpoints, ep)
		}
	})

	rootObj.Apps = apps

	return rootObj, nil
}
