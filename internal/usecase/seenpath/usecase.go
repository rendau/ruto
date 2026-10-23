package seenpath

import (
	"context"
	"fmt"
	"strings"

	"github.com/samber/lo"

	"github.com/rendau/ruto/internal/domain/seenpath/model"
	"github.com/rendau/ruto/internal/errs"
)

type Usecase struct {
	svc        ServiceI
	sessionSvc SessionServiceI
}

func New(svc ServiceI, sessionSvc SessionServiceI) *Usecase {
	return &Usecase{
		svc:        svc,
		sessionSvc: sessionSvc,
	}
}

// Report stores what a gateway counted since its previous heartbeat. It is
// called by gateways, not users, so it carries no session check.
func (u *Usecase) Report(ctx context.Context, items []*model.SeenPath) error {
	items = lo.Filter(items, func(item *model.SeenPath, _ int) bool {
		return item != nil && item.AppId != "" && item.EndpointId != "" && item.Method != "" && item.Hits > 0
	})
	if len(items) == 0 {
		return nil
	}

	if err := u.svc.Add(ctx, items); err != nil {
		return fmt.Errorf("svc.Add: %w", err)
	}

	return nil
}

func (u *Usecase) List(ctx context.Context, appId string) ([]*model.SeenPath, error) {
	if !u.sessionSvc.CtxIsAuthorized(ctx) {
		return nil, errs.NotAuthorized
	}
	appId = strings.TrimSpace(appId)
	if appId == "" {
		return nil, errs.IdRequired
	}
	// Read-only: any authorized user may view it, as it exposes only methods
	// and paths, not secrets.

	items, err := u.svc.ListByApp(ctx, appId)
	if err != nil {
		return nil, fmt.Errorf("svc.ListByApp: %w", err)
	}

	return items, nil
}

// Delete removes single rows, e.g. paths already registered as explicit
// endpoints: they still count against the per-endpoint row cap.
func (u *Usecase) Delete(ctx context.Context, appId string, keys []*model.SeenPathKey) error {
	appId, err := u.checkWrite(ctx, appId)
	if err != nil {
		return err
	}

	keys = lo.Filter(keys, func(v *model.SeenPathKey, _ int) bool {
		return v != nil && v.EndpointId != "" && v.Method != ""
	})
	if len(keys) == 0 {
		return nil
	}

	if err = u.svc.DeleteItems(ctx, appId, keys); err != nil {
		return fmt.Errorf("svc.DeleteItems: %w", err)
	}

	return nil
}

func (u *Usecase) Clear(ctx context.Context, appId string) error {
	appId, err := u.checkWrite(ctx, appId)
	if err != nil {
		return err
	}

	if err = u.svc.DeleteByApp(ctx, appId); err != nil {
		return fmt.Errorf("svc.DeleteByApp: %w", err)
	}

	return nil
}

// checkWrite returns the trimmed app id when the caller may change the app's report.
func (u *Usecase) checkWrite(ctx context.Context, appId string) (string, error) {
	if !u.sessionSvc.CtxIsAuthorized(ctx) {
		return "", errs.NotAuthorized
	}
	appId = strings.TrimSpace(appId)
	if appId == "" {
		return "", errs.IdRequired
	}
	if !u.sessionSvc.CtxHasFullAppAccess(ctx) && !lo.Contains(u.sessionSvc.CtxGetAppIds(ctx), appId) {
		return "", errs.NoPermission
	}
	return appId, nil
}
