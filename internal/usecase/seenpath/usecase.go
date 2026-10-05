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

func (u *Usecase) Clear(ctx context.Context, appId string) error {
	if !u.sessionSvc.CtxIsAuthorized(ctx) {
		return errs.NotAuthorized
	}
	appId = strings.TrimSpace(appId)
	if appId == "" {
		return errs.IdRequired
	}
	if !u.sessionSvc.CtxHasFullAppAccess(ctx) && !lo.Contains(u.sessionSvc.CtxGetAppIds(ctx), appId) {
		return errs.NoPermission
	}

	if err := u.svc.DeleteByApp(ctx, appId); err != nil {
		return fmt.Errorf("svc.DeleteByApp: %w", err)
	}

	return nil
}
