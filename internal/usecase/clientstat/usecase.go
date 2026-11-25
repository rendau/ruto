// Package clientstat shows who calls the gateway: per-client request counters
// the gateways report in heartbeats, kept by core in 5-minute buckets.
package clientstat

import (
	"context"
	"fmt"
	"log/slog"
	"net/netip"
	"strings"
	"time"

	"github.com/samber/lo"

	blockedClientModel "github.com/rendau/ruto/internal/domain/blockedclient/model"
	"github.com/rendau/ruto/internal/domain/clientstat/model"
	rootModel "github.com/rendau/ruto/internal/domain/root/model"
	"github.com/rendau/ruto/internal/errs"
)

const (
	defaultLimit = 50
	maxLimit     = 200

	// maxReportRows and maxFieldLen bound what one heartbeat may write: the
	// gateways already send a bounded report, this keeps a broken one from
	// flooding the table.
	maxReportRows = 5000
	maxFieldLen   = 256
)

type Usecase struct {
	svc        ServiceI
	blockedSvc BlockedClientServiceI
	sessionSvc SessionServiceI

	now func() time.Time
}

func New(svc ServiceI, blockedSvc BlockedClientServiceI, sessionSvc SessionServiceI) *Usecase {
	return &Usecase{
		svc:        svc,
		blockedSvc: blockedSvc,
		sessionSvc: sessionSvc,
		now:        time.Now,
	}
}

// Report stores what a gateway counted since its previous heartbeat. It is
// called by gateways, not users, so it carries no session check.
func (u *Usecase) Report(ctx context.Context, items []*model.ClientStat) error {
	items = lo.Filter(items, func(v *model.ClientStat, _ int) bool {
		return v != nil &&
			v.Requests >= 0 && v.Blocked >= 0 && v.Limited >= 0 &&
			v.Requests+v.Blocked+v.Limited > 0 &&
			len(v.Ip) <= maxFieldLen && len(v.Trusted) <= maxFieldLen && len(v.LastRule) <= maxFieldLen
	})
	if len(items) == 0 {
		return nil
	}
	if len(items) > maxReportRows {
		slog.Warn("client stat: report truncated", "rows", len(items), "kept", maxReportRows)
		items = items[:maxReportRows]
	}

	if err := u.svc.Add(ctx, u.now(), items); err != nil {
		return fmt.Errorf("svc.Add: %w", err)
	}

	return nil
}

func (u *Usecase) Overview(ctx context.Context, rng time.Duration) (*model.Overview, error) {
	if err := u.requireAdmin(ctx); err != nil {
		return nil, err
	}

	result, err := u.svc.Overview(ctx, model.NewWindow(u.now(), rng))
	if err != nil {
		return nil, fmt.Errorf("svc.Overview: %w", err)
	}

	return result, nil
}

func (u *Usecase) Top(ctx context.Context, req *TopReq) ([]*TopItem, error) {
	if err := u.requireAdmin(ctx); err != nil {
		return nil, err
	}

	kind := lo.CoalesceOrEmpty(strings.TrimSpace(req.Kind), model.KindRequests)
	if !lo.Contains([]string{model.KindRequests, model.KindBlocked, model.KindLimited, model.KindTrusted}, kind) {
		return nil, errs.ErrFull{Err: errs.InvalidRequest, Desc: "kind: one of requests, blocked, limited, trusted"}
	}

	now := u.now()

	items, err := u.svc.Top(ctx, &model.TopReq{
		Since: model.NewWindow(now, req.Range).From,
		Kind:  kind,
		Limit: clampLimit(req.Limit),
	})
	if err != nil {
		return nil, fmt.Errorf("svc.Top: %w", err)
	}

	blocks, _, err := u.blockedSvc.List(ctx, &blockedClientModel.ListReq{ActiveAt: &now})
	if err != nil {
		return nil, fmt.Errorf("blockedSvc.List: %w", err)
	}
	matcher := newBlockMatcher(blocks)

	return lo.Map(items, func(v *model.Item, _ int) *TopItem {
		return &TopItem{Item: v, Block: matcher.find(v.Ip)}
	}), nil
}

// Series is the timeline of one client; ip is as Top returns it, "" included
// (the clients the gateways summed together).
func (u *Usecase) Series(ctx context.Context, ip string, rng time.Duration) (*model.Series, error) {
	if err := u.requireAdmin(ctx); err != nil {
		return nil, err
	}

	result, err := u.svc.Series(ctx, model.NewWindow(u.now(), rng), strings.TrimSpace(ip))
	if err != nil {
		return nil, fmt.Errorf("svc.Series: %w", err)
	}

	return result, nil
}

// Prune deletes the buckets older than model.Retention.
func (u *Usecase) Prune(ctx context.Context) {
	n, err := u.svc.DeleteBefore(ctx, u.now().Add(-model.Retention))
	if err != nil {
		slog.Error("client stat: prune", "error", err)
		return
	}
	if n > 0 {
		slog.Info("client stat: old buckets pruned", "rows", n)
	}
}

// RunPrune prunes every interval until ctx is done. Several core replicas
// pruning at once is harmless.
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

// requireAdmin is the rule of BlockedClient.List: the page shows client
// addresses and the block list, so only administrators see it.
func (u *Usecase) requireAdmin(ctx context.Context) error {
	session := u.sessionSvc.FromContext(ctx)
	if !session.IsAuthorized() {
		return errs.NotAuthorized
	}
	if !session.IsAdmin() {
		return errs.NoPermission
	}
	return nil
}

func clampLimit(limit int64) int64 {
	if limit <= 0 {
		return defaultLimit
	}
	return min(limit, maxLimit)
}

type parsedBlock struct {
	prefix netip.Prefix
	block  *blockedClientModel.BlockedClient
}

// blockMatcher finds the active block covering a client of the report: an
// address, an IPv6 /64 or a trusted network.
type blockMatcher struct {
	blocks []parsedBlock
}

func newBlockMatcher(blocks []*blockedClientModel.BlockedClient) *blockMatcher {
	return &blockMatcher{
		blocks: lo.FilterMap(blocks, func(v *blockedClientModel.BlockedClient, _ int) (parsedBlock, bool) {
			prefix, err := rootModel.ParseClientPrefix(v.Ip)
			return parsedBlock{prefix: prefix, block: v}, err == nil
		}),
	}
}

// find returns the narrowest block that contains the whole client network,
// nil when none does or ip is not an address.
func (m *blockMatcher) find(ip string) *blockedClientModel.BlockedClient {
	if ip == "" {
		return nil
	}
	client, err := rootModel.ParseClientPrefix(ip)
	if err != nil {
		return nil
	}

	var best *parsedBlock
	for i := range m.blocks {
		b := &m.blocks[i]
		if b.prefix.Bits() > client.Bits() || !b.prefix.Contains(client.Addr()) {
			continue
		}
		if best == nil || b.prefix.Bits() > best.prefix.Bits() {
			best = b
		}
	}
	if best == nil {
		return nil
	}
	return best.block
}
