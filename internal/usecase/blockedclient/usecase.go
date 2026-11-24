// Package blockedclient is the gateway-wide block list of client addresses.
// Unlike the configuration it applies at once: no change request, no publish.
// Callers with an api key (MCP agents, services such as pulse on the REST API)
// get narrower rights than a person in the admin UI — see guards.go.
package blockedclient

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/samber/lo"

	"github.com/rendau/ruto/internal/domain/blockedclient/model"
	commonModel "github.com/rendau/ruto/internal/domain/common/model"
	rootModel "github.com/rendau/ruto/internal/domain/root/model"
	sessionModel "github.com/rendau/ruto/internal/domain/session/model"
	"github.com/rendau/ruto/internal/errs"
)

const (
	// HistoryRetention — lifted and expired blocks are kept this long.
	HistoryRetention = 90 * 24 * time.Hour

	// historyLimit caps the lifted and expired blocks List returns.
	historyLimit = 1000
)

type Usecase struct {
	svc        ServiceI
	txm        TransactionManagerI
	sessionSvc SessionServiceI
	rootSvc    RootServiceI
	usrSvc     UsrServiceI
	notifier   GatewaysNotifierI

	now func() time.Time
}

func New(
	svc ServiceI,
	txm TransactionManagerI,
	sessionSvc SessionServiceI,
	rootSvc RootServiceI,
	usrSvc UsrServiceI,
	notifier GatewaysNotifierI,
) *Usecase {
	return &Usecase{
		svc:        svc,
		txm:        txm,
		sessionSvc: sessionSvc,
		rootSvc:    rootSvc,
		usrSvc:     usrSvc,
		notifier:   notifier,
		now:        time.Now,
	}
}

// List returns active blocks and, withHistory, the latest lifted and expired
// ones, newest first.
func (u *Usecase) List(ctx context.Context, withHistory bool) ([]*model.BlockedClient, error) {
	if _, err := u.requireAdmin(ctx); err != nil {
		return nil, err
	}

	now := u.now()
	sort := []string{"-created_at", "id"}

	items, _, err := u.svc.List(ctx, &model.ListReq{ActiveAt: &now, ListParams: commonModel.ListParams{Sort: sort}})
	if err != nil {
		return nil, fmt.Errorf("svc.List: %w", err)
	}

	if withHistory {
		history, _, err := u.svc.List(ctx, &model.ListReq{
			InactiveAt: &now,
			ListParams: commonModel.ListParams{PageSize: historyLimit, Sort: sort},
		})
		if err != nil {
			return nil, fmt.Errorf("svc.List: %w", err)
		}
		items = append(items, history...)
		slices.SortStableFunc(items, func(a, b *model.BlockedClient) int {
			return cmp.Or(b.CreatedAt.Compare(a.CreatedAt), strings.Compare(a.Id, b.Id))
		})
	}

	return items, nil
}

// Create blocks a network, or prolongs the active block of the same network:
// it ends at the later of the two expiries (a block without end stays so),
// the reason is replaced. Nobody shortens a block this way.
func (u *Usecase) Create(ctx context.Context, req *CreateReq) (*model.BlockedClient, error) {
	session, err := u.requireAdmin(ctx)
	if err != nil {
		return nil, err
	}
	source := sourceOf(session)

	reason := strings.TrimSpace(req.Reason)
	if err = checkReason(reason, true); err != nil {
		return nil, err
	}
	if err = checkDuration(source, req.Duration); err != nil {
		return nil, err
	}
	prefix, err := rootModel.ParseClientPrefix(req.Ip)
	if err != nil {
		return nil, invalid("ip: " + err.Error())
	}
	if err = checkWidth(source, prefix); err != nil {
		return nil, err
	}
	if err = checkSpecial(source, prefix); err != nil {
		return nil, err
	}
	root, err := u.rootSvc.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("rootSvc.Get: %w", err)
	}
	if err = checkTrusted(root, prefix); err != nil {
		return nil, err
	}

	now := u.now()

	var expiresAt *time.Time
	if req.Duration > 0 {
		expiresAt = new(now.Add(req.Duration))
	}
	ip := prefix.String()
	actor := u.actor(ctx, session)

	var (
		id      string
		outcome string
	)
	err = u.txm.TxFn(ctx, func(ctx context.Context) error {
		if err := u.svc.Lock(ctx); err != nil {
			return err
		}
		if err := u.svc.CloseExpired(ctx, ip, now); err != nil {
			return err
		}

		existing, found, err := u.svc.GetNotRemoved(ctx, ip)
		if err != nil {
			return err
		}
		if found {
			id = existing.Id
			// an agent never rewrites a person's record: the block already
			// holds, nothing to do
			if source == model.SourceAgent && existing.Source != model.SourceAgent {
				outcome = outcomeUnchanged
				return nil
			}
			outcome = outcomeProlonged
			return u.svc.Update(ctx, existing.Id, prolongEdit(existing, source, reason, expiresAt, now))
		}

		if source == model.SourceAgent {
			if err := u.checkAgentNew(ctx, prefix, now); err != nil {
				return err
			}
		} else {
			count, err := u.svc.CountActive(ctx, now, new(model.SourceAdmin))
			if err != nil {
				return err
			}
			if count >= MaxActive {
				return invalid(fmt.Sprintf("there are already %d active blocks made by people, the most allowed; lift some first", count))
			}
		}

		outcome = outcomeCreated
		id, err = u.svc.Create(ctx, &model.BlockedClient{
			Ip:        ip,
			Reason:    reason,
			CreatedBy: actor,
			Source:    source,
			ApiKeyId:  lo.Ternary(source == model.SourceAgent, session.ApiKeyId, ""),
			CreatedAt: now,
			ExpiresAt: expiresAt,
		})
		return err
	})
	if err != nil {
		return nil, unwrapTx(err)
	}

	item, _, err := u.svc.Get(ctx, id, true)
	if err != nil {
		return nil, fmt.Errorf("svc.Get: %w", err)
	}

	slog.Info("blocked client: "+outcome, "ip", item.Ip, "source", item.Source, "expires_at", item.ExpiresAt,
		"reason", item.Reason, "by", actor, "caller_source", source)
	if outcome != outcomeUnchanged {
		u.notifier.NotifyBlocklist()
	}

	return item, nil
}

const (
	outcomeCreated   = "blocked"
	outcomeProlonged = "prolonged"
	outcomeUnchanged = "already blocked by a person, unchanged"
)

// checkAgentNew holds the limits on new agent blocks. They are counted in the
// database under the write lock, so they are shared by all keys, survive
// restarts and hold across core replicas.
func (u *Usecase) checkAgentNew(ctx context.Context, prefix netip.Prefix, now time.Time) error {
	lifted, err := u.svc.ListLiftedByAdmin(ctx, now.Add(-AgentCooldownAfterLift))
	if err != nil {
		return err
	}
	for _, item := range lifted {
		liftedPrefix, err := rootModel.ParseClientPrefix(item.Ip)
		if err != nil || !liftedPrefix.Overlaps(prefix) {
			continue
		}
		return noPermission(fmt.Sprintf("a person lifted the block of %s on %s (reason: %q); agents may not block an overlapping network for %s after that",
			item.Ip, item.RemovedAt.Format(time.RFC3339), item.RemoveReason, AgentCooldownAfterLift))
	}

	active, err := u.svc.CountActive(ctx, now, new(model.SourceAgent))
	if err != nil {
		return err
	}
	if active >= MaxAgentActive {
		return invalid(fmt.Sprintf("agents already hold %d active blocks, the most allowed for agents; ask a person", active))
	}

	recent, err := u.svc.CountCreatedSince(ctx, model.SourceAgent, now.Add(-time.Hour))
	if err != nil {
		return err
	}
	if recent >= AgentNewBlocksPerHour {
		return invalid(fmt.Sprintf("agents (all keys together) already made %d new blocks in the last hour, the most allowed; wait or ask a person", recent))
	}

	return nil
}

// prolongEdit: the block ends at the later of the two expiries; an agent
// cannot carry its block past created_at + AgentMaxBlockAge.
func prolongEdit(existing *model.BlockedClient, source, reason string, expiresAt *time.Time, now time.Time) *model.Edit {
	if source == model.SourceAgent && expiresAt != nil {
		if limit := existing.CreatedAt.Add(AgentMaxBlockAge); expiresAt.After(limit) {
			expiresAt = &limit
		}
	}

	edit := &model.Edit{Reason: &reason, UpdatedAt: &now}
	if later := laterExpiry(existing.ExpiresAt, expiresAt); !sameExpiry(later, existing.ExpiresAt) {
		edit.SetExpiresAt = true
		edit.ExpiresAt = later
	}
	// a person confirmed the block: an agent may not lift it anymore
	if source == model.SourceAdmin && existing.Source != model.SourceAdmin {
		edit.Source = new(model.SourceAdmin)
	}
	return edit
}

// Remove lifts an active block; the row stays in the history. A caller with
// an api key lifts only blocks made by agents: an agent must not undo a
// person's block.
func (u *Usecase) Remove(ctx context.Context, req *RemoveReq) (*model.BlockedClient, error) {
	session, err := u.requireAdmin(ctx)
	if err != nil {
		return nil, err
	}
	source := sourceOf(session)

	reason := strings.TrimSpace(req.Reason)
	if err = checkReason(reason, source == model.SourceAgent); err != nil {
		return nil, err
	}

	id := strings.TrimSpace(req.Id)
	ip := ""
	if id == "" {
		if strings.TrimSpace(req.Ip) == "" {
			return nil, invalid("id or ip is required")
		}
		prefix, err := rootModel.ParseClientPrefix(req.Ip)
		if err != nil {
			return nil, invalid("ip: " + err.Error())
		}
		ip = prefix.String()
	}

	now := u.now()
	actor := u.actor(ctx, session)

	err = u.txm.TxFn(ctx, func(ctx context.Context) error {
		if err := u.svc.Lock(ctx); err != nil {
			return err
		}

		var item *model.BlockedClient
		if id != "" {
			found, _, err := u.svc.Get(ctx, id, true)
			if err != nil {
				return err
			}
			item = found
		} else {
			found, ok, err := u.svc.GetNotRemoved(ctx, ip)
			if err != nil {
				return err
			}
			if !ok {
				return errs.ErrFull{Err: errs.ObjectNotFound, Desc: "no active block of exactly " + ip}
			}
			item = found
		}

		if !item.IsActive(now) {
			return invalid("the block of " + item.Ip + " is not active: already lifted or expired")
		}
		if source == model.SourceAgent && item.Source != model.SourceAgent {
			return noPermission("the block of " + item.Ip + " was made or confirmed by a person; only a person can lift it in the ruto admin UI")
		}

		id = item.Id
		return u.svc.Update(ctx, item.Id, &model.Edit{
			RemovedAt:     &now,
			RemovedBy:     &actor,
			RemoveReason:  &reason,
			RemovedSource: &source,
		})
	})
	if err != nil {
		return nil, unwrapTx(err)
	}

	slog.Info("blocked client: lifted", "id", id, "source", source, "by", actor, "reason", reason)
	u.notifier.NotifyBlocklist()

	item, _, err := u.svc.Get(ctx, id, true)
	if err != nil {
		return nil, fmt.Errorf("svc.Get: %w", err)
	}
	return item, nil
}

// RemoveAgentBlocks is the kill switch: it lifts every active agent block, or
// only those of apiKeyId. Only a person logged into the admin UI may use it,
// never a key. Lifted this way counts as lifted by a person (the agents'
// cooldown applies).
func (u *Usecase) RemoveAgentBlocks(ctx context.Context, apiKeyId, reason string) (int64, error) {
	session := u.sessionSvc.FromContext(ctx)
	if !session.IsAuthorized() {
		return 0, errs.NotAuthorized
	}
	if !session.IsHuman() || !session.IsAdmin() {
		return 0, errs.NoPermission
	}

	reason = strings.TrimSpace(reason)
	if err := checkReason(reason, true); err != nil {
		return 0, err
	}

	apiKeyId = strings.TrimSpace(apiKeyId)
	return u.removeMany(ctx, &model.RemoveMany{
		ApiKeyId:      lo.Ternary(apiKeyId == "", nil, &apiKeyId),
		RemovedBy:     u.actor(ctx, session),
		RemoveReason:  reason,
		RemovedSource: model.SourceAdmin,
	})
}

// RemoveKeyBlocks lifts the active blocks of a deleted api key. The caller
// (usecase/apikey) has checked the rights; a deleted key may have been
// rotated, not wrong, so no cooldown follows.
func (u *Usecase) RemoveKeyBlocks(ctx context.Context, apiKeyId, keyName string) (int64, error) {
	if apiKeyId == "" {
		return 0, nil
	}
	return u.removeMany(ctx, &model.RemoveMany{
		ApiKeyId:      &apiKeyId,
		RemovedBy:     u.actor(ctx, u.sessionSvc.FromContext(ctx)),
		RemoveReason:  "api key " + keyName + " deleted",
		RemovedSource: model.RemovedSourceKeyDeleted,
	})
}

func (u *Usecase) removeMany(ctx context.Context, req *model.RemoveMany) (int64, error) {
	req.Now = u.now()

	var n int64
	err := u.txm.TxFn(ctx, func(ctx context.Context) error {
		if err := u.svc.Lock(ctx); err != nil {
			return err
		}
		var err error
		n, err = u.svc.RemoveMany(ctx, req)
		return err
	})
	if err != nil {
		return 0, unwrapTx(err)
	}

	slog.Info("blocked client: agent blocks lifted", "count", n, "api_key_id", lo.FromPtr(req.ApiKeyId),
		"by", req.RemovedBy, "reason", req.RemoveReason)
	if n > 0 {
		u.notifier.NotifyBlocklist()
	}
	return n, nil
}

// GetVersion and GetActive serve the gateways over the internal gRPC port
// and, like Snapshot.Get, carry no session check.

func (u *Usecase) GetVersion(ctx context.Context) (string, error) {
	_, version, err := u.svc.Active(ctx, u.now())
	if err != nil {
		return "", fmt.Errorf("svc.Active: %w", err)
	}
	return version, nil
}

func (u *Usecase) GetActive(ctx context.Context) ([]*model.Rule, string, error) {
	rules, version, err := u.svc.Active(ctx, u.now())
	if err != nil {
		return nil, "", fmt.Errorf("svc.Active: %w", err)
	}
	return rules, version, nil
}

// Prune deletes the history of blocks that ended more than HistoryRetention ago.
func (u *Usecase) Prune(ctx context.Context) {
	n, err := u.svc.Prune(ctx, u.now().Add(-HistoryRetention))
	if err != nil {
		slog.Error("blocked client: prune", "error", err)
		return
	}
	if n > 0 {
		slog.Info("blocked client: history pruned", "rows", n)
	}
}

func (u *Usecase) RunPrune(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		u.Prune(ctx)

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (u *Usecase) requireAdmin(ctx context.Context) (*sessionModel.Session, error) {
	session := u.sessionSvc.FromContext(ctx)
	if !session.IsAuthorized() {
		return nil, errs.NotAuthorized
	}
	if !session.IsAdmin() {
		return nil, errs.NoPermission
	}
	return session, nil
}

// sourceOf: a session with an api key or OAuth grant is an agent (MCP or the
// REST API with a key), otherwise a person in the admin UI.
func sourceOf(session *sessionModel.Session) string {
	if session.IsHuman() {
		return model.SourceAdmin
	}
	return model.SourceAgent
}

// actor names the caller for created_by / removed_by: the user, and for an
// agent also its api key ("Ivan (key pulse)") or OAuth client
// ("Ivan (Claude (OAuth))").
func (u *Usecase) actor(ctx context.Context, session *sessionModel.Session) string {
	name := "#" + strconv.FormatInt(session.Id, 10)
	if usr, found, err := u.usrSvc.Get(ctx, session.Id, false); err == nil && found {
		name = cmp.Or(usr.Name, usr.Username, name)
	}

	if session.IsHuman() {
		return name
	}
	if strings.HasSuffix(session.ApiKeyName, "(OAuth)") {
		return name + " (" + session.ApiKeyName + ")"
	}
	return name + " (key " + session.ApiKeyName + ")"
}

// unwrapTx strips the transaction wrapper of mobone so sentinel errors and
// their descriptions reach the caller unchanged.
func unwrapTx(err error) error {
	if full, ok := errors.AsType[errs.ErrFull](err); ok {
		return full
	}
	if base, ok := errors.AsType[errs.Err](err); ok {
		return base
	}
	return err
}
