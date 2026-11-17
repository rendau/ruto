package snapshot

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"

	"encoding/json"

	"github.com/samber/lo"

	appModel "github.com/rendau/ruto/internal/domain/app/model"
	endpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
	rootModel "github.com/rendau/ruto/internal/domain/root/model"
	snapshotModel "github.com/rendau/ruto/internal/domain/snapshot/model"
	"github.com/rendau/ruto/internal/errs"
)

type Usecase struct {
	snapshotSvc ServiceI
	rootSvc     RootServiceI
	appSvc      AppServiceI
	endpointSvc EndpointServiceI
	gateways    GatewaysNotifierI
	sessionSvc  SessionServiceI
}

func New(
	snapshotSvc ServiceI,
	rootSvc RootServiceI,
	appSvc AppServiceI,
	endpointSvc EndpointServiceI,
	gateways GatewaysNotifierI,
	sessionSvc SessionServiceI,
) *Usecase {
	result := &Usecase{
		snapshotSvc: snapshotSvc,
		rootSvc:     rootSvc,
		appSvc:      appSvc,
		endpointSvc: endpointSvc,
		gateways:    gateways,
		sessionSvc:  sessionSvc,
	}

	if err := result.ensure(context.Background()); err != nil {
		slog.Error("snapshot-usecase: ensure failed", "error", err)
	}

	return result
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

// Deploy publishes the saved config to gateways. Any logged-in user may run
// it: the button is shown to every user, and it only publishes what editors
// already saved within their own permissions.
func (u *Usecase) Deploy(ctx context.Context) error {
	if !u.sessionSvc.CtxIsAuthorized(ctx) {
		return errs.NotAuthorized
	}

	if err := u.Refresh(ctx); err != nil {
		return fmt.Errorf("Refresh: %w", err)
	}

	return nil
}

func (u *Usecase) Refresh(ctx context.Context) error {
	result, newVersion, err := u.build(ctx)
	if err != nil {
		return err
	}

	return u.store(ctx, result, newVersion)
}

func (u *Usecase) store(ctx context.Context, data []byte, version string) error {
	err := u.snapshotSvc.Set(ctx, &snapshotModel.Snapshot{
		Hash: version,
		Data: data,
	})
	if err != nil {
		return fmt.Errorf("snapshotSvc.Set: %w", err)
	}

	// Push a notification to all connected gateways so they re-check the
	// snapshot version immediately instead of waiting for the next poll.
	u.gateways.NotifyAll()

	return nil
}

// Validate builds the snapshot from the current configuration (in ctx's
// transaction, if any) without storing it: a configuration that would fail
// here would block publishing for every app.
func (u *Usecase) Validate(ctx context.Context) error {
	_, _, err := u.build(ctx)
	return err
}

// CurrentVersion is the version a publish would produce now; it differs from
// GetVersion while there are unpublished changes.
func (u *Usecase) CurrentVersion(ctx context.Context) (string, error) {
	_, version, err := u.build(ctx)
	return version, err
}

func (u *Usecase) build(ctx context.Context) ([]byte, string, error) {
	snapshotObj, err := u.construct(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("construct: %w", err)
	}

	err = snapshotObj.Normalize()
	if err != nil {
		return nil, "", fmt.Errorf("snapshot.Normalize: %w", err)
	}

	result, err := json.Marshal(snapshotObj)
	if err != nil {
		return nil, "", fmt.Errorf("json.Marshal: %w", err)
	}

	return result, digest(result), nil
}

func digest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func (u *Usecase) ensure(ctx context.Context) error {
	current, err := u.snapshotSvc.Get(ctx)
	if err != nil {
		return fmt.Errorf("snapshotSvc.Get: %w", err)
	}
	if current.Hash == "" {
		if err = u.Refresh(ctx); err != nil {
			return fmt.Errorf("u.Refresh: %w", err)
		}
		return nil
	}

	// A migration may edit the published snapshot in SQL. Its hash is then
	// recomputed from the stored data: gateways reload it, and publishing the
	// unchanged config later still yields the same version.
	obj := &rootModel.Root{}
	if err = json.Unmarshal(current.Data, obj); err != nil {
		return fmt.Errorf("json.Unmarshal: %w", err)
	}
	data, err := json.Marshal(obj)
	if err != nil {
		return fmt.Errorf("json.Marshal: %w", err)
	}
	if version := digest(data); version != current.Hash {
		slog.Info("snapshot-usecase: stored snapshot changed, version recomputed", "old", current.Hash, "new", version)
		if err = u.store(ctx, data, version); err != nil {
			return fmt.Errorf("store: %w", err)
		}
	}

	return nil
}

func (u *Usecase) construct(ctx context.Context) (*rootModel.Root, error) {
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
