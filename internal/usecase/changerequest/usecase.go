package changerequest

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/rendau/ruto/internal/domain/changerequest/model"
	"github.com/rendau/ruto/internal/domain/secretref"
	"github.com/rendau/ruto/internal/errs"
)

// errDryRun rolls back the transaction of a proposal check.
var errDryRun = errors.New("dry run")

type Usecase struct {
	conf       Config
	svc        ServiceI
	txm        TransactionManagerI
	sessionSvc SessionServiceI

	rootUc     RootUsecaseI
	appUc      AppUsecaseI
	endpointUc EndpointUsecaseI
	seenPathUc SeenPathUsecaseI
	snapshotUc SnapshotUsecaseI

	rootSvc     RootServiceI
	appSvc      AppServiceI
	endpointSvc EndpointServiceI
	seenPathSvc SeenPathServiceI
	usrSvc      UsrServiceI
}

func New(
	conf Config,
	svc ServiceI,
	txm TransactionManagerI,
	sessionSvc SessionServiceI,
	rootUc RootUsecaseI,
	appUc AppUsecaseI,
	endpointUc EndpointUsecaseI,
	seenPathUc SeenPathUsecaseI,
	snapshotUc SnapshotUsecaseI,
	rootSvc RootServiceI,
	appSvc AppServiceI,
	endpointSvc EndpointServiceI,
	seenPathSvc SeenPathServiceI,
	usrSvc UsrServiceI,
) *Usecase {
	return &Usecase{
		conf:        conf,
		svc:         svc,
		txm:         txm,
		sessionSvc:  sessionSvc,
		rootUc:      rootUc,
		appUc:       appUc,
		endpointUc:  endpointUc,
		seenPathUc:  seenPathUc,
		snapshotUc:  snapshotUc,
		rootSvc:     rootSvc,
		appSvc:      appSvc,
		endpointSvc: endpointSvc,
		seenPathSvc: seenPathSvc,
		usrSvc:      usrSvc,
	}
}

// ── Agent side ──────────────────────────────────────────

// Propose checks the operations by running them in a transaction that is
// always rolled back, then stores them as a pending request.
func (u *Usecase) Propose(ctx context.Context, req *ProposeReq) (int64, error) {
	session := u.sessionSvc.FromContext(ctx)
	if !session.IsAdmin() || session.ApiKeyId == "" {
		return 0, errs.NoPermission
	}

	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		return 0, invalid("title is required: say what the request does and why")
	}

	_, pending, err := u.svc.List(ctx, &model.ListReq{
		ListParams: listOnlyCount(),
		Status:     new(model.StatusPending),
		ApiKeyId:   &session.ApiKeyId,
	})
	if err != nil {
		return 0, fmt.Errorf("svc.List: %w", err)
	}
	if pending >= u.conf.MaxPendingPerKey {
		return 0, invalid(fmt.Sprintf("this key already has %d undecided requests; wait for decisions or cancel some", pending))
	}

	ops := make([]*model.Op, len(req.Ops))
	for i, op := range req.Ops {
		ops[i] = &op.Op
	}
	if err = checkShape(ops); err != nil {
		return 0, err
	}
	for i, op := range ops {
		if op.Op != model.OpDeleteSeenPaths {
			continue
		}
		if err = u.prepareSeenPaths(ctx, op); err != nil {
			return 0, wrapOp(i, op, err)
		}
	}

	plan := model.Plan{Ops: ops, Versions: map[string]string{}}

	err = u.txm.TxFn(ctx, func(ctx context.Context) error {
		cache := u.newObjectCache()

		for i, op := range req.Ops {
			kind, id, ok := opTarget(&op.Op)
			if !ok {
				continue
			}
			item, found, err := cache.get(ctx, kind, id)
			if err != nil {
				return err
			}
			if !found {
				return wrapOp(i, &op.Op, errNotFound(kind, id))
			}
			if op.Version == "" {
				return wrapOp(i, &op.Op, invalid("version is required: pass the version you read the object with"))
			}
			if op.Version != item.Version {
				return wrapOp(i, &op.Op, outdated("the object changed since you read it; read it again"))
			}
			plan.Versions[item.Key()] = item.Version
		}

		refs, err := collectRefs(ops)
		if err != nil {
			return err
		}
		for key, ref := range refs {
			item, found, err := cache.get(ctx, ref.Kind, ref.Id)
			if err != nil {
				return err
			}
			if !found {
				return invalid("secret_ref " + ref.String() + ": object not found")
			}
			plan.Versions[key] = item.Version
		}

		if _, err = u.applyPlan(ctx, &plan); err != nil {
			return err
		}

		return errDryRun
	})
	if err != nil && !errors.Is(err, errDryRun) {
		return 0, unwrapTx(err)
	}

	newId, err := u.svc.Create(ctx, &model.ChangeRequest{
		Title:       req.Title,
		Description: strings.TrimSpace(req.Description),
		Publish:     req.Publish,
		Plan:        plan,
		AuthorUsrId: session.Id,
		ApiKeyId:    session.ApiKeyId,
		ApiKeyName:  session.ApiKeyName,
	})
	if err != nil {
		return 0, fmt.Errorf("svc.Create: %w", err)
	}

	return newId, nil
}

// Cancel withdraws a pending request made with the caller's key.
func (u *Usecase) Cancel(ctx context.Context, id int64) error {
	session := u.sessionSvc.FromContext(ctx)
	if !session.IsAdmin() || session.ApiKeyId == "" {
		return errs.NoPermission
	}

	return unwrapTx(u.txm.TxFn(ctx, func(ctx context.Context) error {
		item, err := u.svc.GetLocked(ctx, id)
		if err != nil {
			return err
		}
		if item.ApiKeyId != session.ApiKeyId {
			return errs.NoPermission
		}
		if item.IsClosed() {
			return errs.ChangeRequestClosed
		}
		review, err := u.review(ctx, item)
		if err != nil {
			return err
		}
		return u.svc.Update(ctx, id, &model.Edit{
			Status:    new(model.StatusCancelled),
			DecidedAt: new(time.Now()),
			Result:    &model.Result{Review: review},
		})
	}))
}

// ── Shared reads ────────────────────────────────────────

func (u *Usecase) List(ctx context.Context, pars *model.ListReq) ([]*model.ChangeRequest, int64, error) {
	session := u.sessionSvc.FromContext(ctx)
	if !session.IsAdmin() {
		return nil, 0, errs.NoPermission
	}
	// an agent sees only the requests of its own key
	if session.ApiKeyId != "" {
		pars.ApiKeyId = &session.ApiKeyId
	}

	items, tCount, err := u.svc.List(ctx, pars)
	if err != nil {
		return nil, 0, fmt.Errorf("svc.List: %w", err)
	}
	return items, tCount, nil
}

func (u *Usecase) Get(ctx context.Context, id int64) (*Detail, error) {
	session := u.sessionSvc.FromContext(ctx)
	if !session.IsAdmin() {
		return nil, errs.NoPermission
	}

	item, _, err := u.svc.Get(ctx, id, true)
	if err != nil {
		return nil, fmt.Errorf("svc.Get: %w", err)
	}
	if session.ApiKeyId != "" && item.ApiKeyId != session.ApiKeyId {
		return nil, errs.NoPermission
	}

	return u.detail(ctx, item)
}

// ── Admin side ──────────────────────────────────────────

// Apply runs the request in one transaction: all operations or none. Only a
// person logged into the admin UI may apply; an api key never can.
func (u *Usecase) Apply(ctx context.Context, id int64, publish bool) error {
	session := u.sessionSvc.FromContext(ctx)
	if !session.IsHuman() || !session.IsAdmin() {
		return errs.NoPermission
	}

	var closeStatus string

	err := u.txm.TxFn(ctx, func(ctx context.Context) error {
		item, err := u.svc.GetLocked(ctx, id)
		if err != nil {
			return err
		}
		if item.IsClosed() {
			return errs.ChangeRequestClosed
		}
		if time.Since(item.CreatedAt) > u.conf.PendingTTL {
			closeStatus = model.StatusExpired
			return errs.ChangeRequestExpired
		}

		review, err := u.review(ctx, item)
		if err != nil {
			return err
		}

		result, err := u.applyPlan(ctx, &item.Plan)
		if err != nil {
			if base, _ := errsBase(err); base == errs.ChangeRequestOutdated {
				closeStatus = model.StatusOutdated
			}
			return err
		}
		result.Review = review

		return u.svc.Update(ctx, id, &model.Edit{
			Status:       new(model.StatusApplied),
			DecidedAt:    new(time.Now()),
			DecidedUsrId: &session.Id,
			Result:       result,
		})
	})
	if err != nil {
		err = unwrapTx(err)
		if closeStatus != "" {
			u.close(ctx, id, closeStatus, session.Id, errDesc(err))
		}
		return err
	}

	if publish {
		edit := &model.Edit{PublishedAt: new(time.Now())}
		if err = u.snapshotUc.Refresh(ctx); err != nil {
			edit = &model.Edit{PublishError: new(err.Error())}
		}
		if updErr := u.svc.Update(ctx, id, edit); updErr != nil {
			slog.Error("change request: store publish result", "error", updErr, "id", id)
		}
		if err != nil {
			return errs.ErrFull{Err: errs.ServiceNA, Desc: "changes applied, but publishing failed: " + err.Error()}
		}
	}

	return nil
}

func (u *Usecase) Reject(ctx context.Context, id int64, comment string) error {
	session := u.sessionSvc.FromContext(ctx)
	if !session.IsHuman() || !session.IsAdmin() {
		return errs.NoPermission
	}

	return unwrapTx(u.txm.TxFn(ctx, func(ctx context.Context) error {
		item, err := u.svc.GetLocked(ctx, id)
		if err != nil {
			return err
		}
		if item.IsClosed() {
			return errs.ChangeRequestClosed
		}
		review, err := u.review(ctx, item)
		if err != nil {
			return err
		}
		return u.svc.Update(ctx, id, &model.Edit{
			Status:       new(model.StatusRejected),
			DecidedAt:    new(time.Now()),
			DecidedUsrId: &session.Id,
			Comment:      new(strings.TrimSpace(comment)),
			Result:       &model.Result{Review: review},
		})
	}))
}

// close records a terminal status found while applying (outdated/expired);
// the apply transaction itself was rolled back.
func (u *Usecase) close(ctx context.Context, id int64, status string, usrId int64, reason string) {
	err := u.svc.Update(ctx, id, &model.Edit{
		Status:       &status,
		DecidedAt:    new(time.Now()),
		DecidedUsrId: &usrId,
		Error:        &reason,
	})
	if err != nil {
		slog.Error("change request: close", "error", err, "id", id, "status", status)
	}
}

// Cleanup expires stale pending requests and deletes old closed ones.
func (u *Usecase) Cleanup(ctx context.Context) {
	expired, deleted, err := u.svc.Cleanup(ctx, u.conf.PendingTTL, u.conf.Retention)
	if err != nil {
		slog.Error("change request: cleanup", "error", err)
		return
	}
	if expired > 0 || deleted > 0 {
		slog.Info("change request: cleanup", "expired", expired, "deleted", deleted)
	}
}

// RunCleanup calls Cleanup now and then every interval until ctx is done.
func (u *Usecase) RunCleanup(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		u.Cleanup(ctx)

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// IsObjectKind reports kinds accepted by View.
func IsObjectKind(kind string) bool {
	return kind == secretref.KindRoot || kind == secretref.KindApp || kind == secretref.KindEndpoint
}

// unwrapTx strips the "transaction function:" wrapper of mobone so sentinel
// errors and their descriptions reach the caller unchanged.
func unwrapTx(err error) error {
	if err == nil {
		return nil
	}
	if full, ok := errors.AsType[errs.ErrFull](err); ok {
		return full
	}
	if base, ok := errors.AsType[errs.Err](err); ok {
		return base
	}
	if msg, ok := strings.CutPrefix(err.Error(), "transaction function: "); ok {
		return errors.New(msg)
	}
	return err
}
