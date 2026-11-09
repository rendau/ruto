package seenorigin

import (
	"context"
	"fmt"
	"strings"

	"github.com/samber/lo"

	"github.com/rendau/ruto/internal/domain/seenorigin/model"
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
func (u *Usecase) Report(ctx context.Context, items []*model.SeenOrigin) error {
	items = lo.Filter(items, func(item *model.SeenOrigin, _ int) bool {
		return item != nil && item.Origin != "" && item.Hits > 0
	})
	if len(items) == 0 {
		return nil
	}

	if err := u.svc.Add(ctx, items); err != nil {
		return fmt.Errorf("svc.Add: %w", err)
	}

	return nil
}

// List is open to any logged-in user: it shows host names only, no secrets.
func (u *Usecase) List(ctx context.Context) ([]*model.SeenOrigin, error) {
	if !u.sessionSvc.CtxIsAuthorized(ctx) {
		return nil, errs.NotAuthorized
	}

	items, err := u.svc.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("svc.List: %w", err)
	}

	return items, nil
}

// Delete removes single rows, e.g. origins already added to allow_origins:
// they still count against the row cap. The report belongs to the root
// configuration, so like it only an administrator may change it.
func (u *Usecase) Delete(ctx context.Context, origins []string) error {
	if err := u.checkWrite(ctx); err != nil {
		return err
	}

	origins = lo.Filter(lo.Map(origins, func(v string, _ int) string { return strings.TrimSpace(v) }),
		func(v string, _ int) bool { return v != "" })
	if len(origins) == 0 {
		return nil
	}

	if err := u.svc.DeleteItems(ctx, origins); err != nil {
		return fmt.Errorf("svc.DeleteItems: %w", err)
	}

	return nil
}

func (u *Usecase) Clear(ctx context.Context) error {
	if err := u.checkWrite(ctx); err != nil {
		return err
	}

	if err := u.svc.DeleteAll(ctx); err != nil {
		return fmt.Errorf("svc.DeleteAll: %w", err)
	}

	return nil
}

func (u *Usecase) checkWrite(ctx context.Context) error {
	if !u.sessionSvc.CtxIsAuthorized(ctx) {
		return errs.NotAuthorized
	}
	if !u.sessionSvc.CtxIsAdmin(ctx) {
		return errs.NoPermission
	}
	return nil
}
