package gateway

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rendau/ruto/internal/errs"
)

type Usecase struct {
	sessionSvc  SessionServiceI
	cache       CacheI
	gateways    GatewaysI
	seenPaths   SeenPathsI
	seenOrigins SeenOriginsI

	// blocklist triggers of the subscribed gateways. The gateways pool fires
	// only "check_version", so these are kept here, next to the streams.
	blocklistMu   sync.Mutex
	blocklistSeq  int64
	blocklistSubs map[int64]func()
}

// Status thresholds are tied to the gateway's heartbeat interval (30s): online
// tolerates one missed beat, stale a few more, after which the gateway is
// considered gone.
const (
	itemTTL         = 10 * time.Minute
	statusOnlineTTL = 70 * time.Second
	statusStaleTTL  = 3 * time.Minute
)

const (
	StatusOnline  = "online"
	StatusStale   = "stale"
	StatusOffline = "offline"
)

func New(sessionSvc SessionServiceI, cache CacheI, gateways GatewaysI, seenPaths SeenPathsI, seenOrigins SeenOriginsI) *Usecase {
	return &Usecase{
		sessionSvc:    sessionSvc,
		cache:         cache,
		gateways:      gateways,
		seenPaths:     seenPaths,
		seenOrigins:   seenOrigins,
		blocklistSubs: map[int64]func(){},
	}
}

// Subscribe registers a connected gateway and blocks for the lifetime of its
// stream. Whenever core wants the gateway to re-check the snapshot version or
// the block list, the matching trigger fires and `send` pushes a notification
// of that type (Notification*) down the stream. The call returns when the
// stream context is done (gateway disconnected) or `send` fails, removing the
// gateway from the pool via the deferred Unregister.
func (u *Usecase) Subscribe(ctx context.Context, gatewayID string, send func(kind string) error) error {
	gatewayID = strings.TrimSpace(gatewayID)

	// buffered+coalescing: redundant triggers collapse into a single pending
	// notification, so a burst of deploys still results in one re-check.
	versionCh := make(chan struct{}, 1)
	blocklistCh := make(chan struct{}, 1)
	trigger := func(ch chan struct{}) func() {
		return func() {
			select {
			case ch <- struct{}{}:
			default:
			}
		}
	}

	id := u.gateways.Register(gatewayID, trigger(versionCh))
	defer u.gateways.Unregister(id)

	blocklistId := u.addBlocklistSub(trigger(blocklistCh))
	defer u.removeBlocklistSub(blocklistId)

	// Ask the gateway to sync right after (re)connect: while it was
	// disconnected, the snapshot and the block list may have changed.
	trigger(versionCh)()
	trigger(blocklistCh)()

	for {
		kind := ""
		select {
		case <-ctx.Done():
			return nil
		case <-u.gateways.Done():
			// core is shutting down — end the stream so GracefulStop unblocks.
			return nil
		case <-versionCh:
			kind = NotificationCheckVersion
		case <-blocklistCh:
			kind = NotificationCheckBlocklist
		}
		if err := send(kind); err != nil {
			return fmt.Errorf("send: %w", err)
		}
	}
}

// NotifyBlocklist makes every subscribed gateway re-check the block list.
func (u *Usecase) NotifyBlocklist() {
	u.blocklistMu.Lock()
	notifiers := make([]func(), 0, len(u.blocklistSubs))
	for _, notify := range u.blocklistSubs {
		notifiers = append(notifiers, notify)
	}
	u.blocklistMu.Unlock()

	for _, notify := range notifiers {
		notify()
	}
}

func (u *Usecase) addBlocklistSub(notify func()) int64 {
	u.blocklistMu.Lock()
	defer u.blocklistMu.Unlock()

	u.blocklistSeq++
	u.blocklistSubs[u.blocklistSeq] = notify
	return u.blocklistSeq
}

func (u *Usecase) removeBlocklistSub(id int64) {
	u.blocklistMu.Lock()
	defer u.blocklistMu.Unlock()

	delete(u.blocklistSubs, id)
}

func (u *Usecase) Heartbeat(ctx context.Context, req *Heartbeat) error {
	if req == nil {
		return errs.InvalidRequest
	}
	gatewayID := strings.TrimSpace(req.GatewayID)
	if gatewayID == "" {
		return errs.InvalidRequest
	}

	item := &Item{
		GatewayID:        gatewayID,
		HostName:         strings.TrimSpace(req.HostName),
		SnapshotVersion:  strings.TrimSpace(req.SnapshotVersion),
		BlocklistVersion: strings.TrimSpace(req.BlocklistVersion),
		LastApplyAtUnix:  req.LastApplyAtUnix,
		StartedAtUnix:    req.StartedAtUnix,
		LastError:        strings.TrimSpace(req.LastError),
		LastSeenAtUnix:   time.Now().Unix(),
		MemoryAllocBytes: req.MemoryAllocBytes,
		GoroutinesCount:  req.GoroutinesCount,
	}

	if err := u.cache.SetJsonObj(gatewayID, item, itemTTL); err != nil {
		return fmt.Errorf("cache.SetJsonObj: %w", err)
	}

	// An error here makes the gateway keep its counters and resend them.
	if err := u.seenPaths.Report(ctx, req.SeenPaths); err != nil {
		return fmt.Errorf("seenPaths.Report: %w", err)
	}
	if err := u.seenOrigins.Report(ctx, req.SeenOrigins); err != nil {
		return fmt.Errorf("seenOrigins.Report: %w", err)
	}

	return nil
}

func (u *Usecase) List(ctx context.Context) ([]*Item, error) {
	if !u.sessionSvc.CtxIsAuthorized(ctx) {
		return nil, errs.NotAuthorized
	}

	return u.ListAll(ctx)
}

// ListAll is List without the login check, for the system port only (the pulse
// manifest): it is not reachable through the API.
func (u *Usecase) ListAll(_ context.Context) ([]*Item, error) {
	keys, err := u.cache.ListKeys()
	if err != nil {
		return nil, fmt.Errorf("cache.ListKeys: %w", err)
	}

	now := time.Now()
	items := make([]*Item, 0, len(keys))
	for _, key := range keys {
		item := new(Item)
		ok, getErr := u.cache.GetJsonObj(key, item)
		if getErr != nil {
			return nil, fmt.Errorf("cache.GetJsonObj(%s): %w", key, getErr)
		}
		if !ok {
			continue
		}

		age := now.Sub(time.Unix(item.LastSeenAtUnix, 0))
		switch {
		case age <= statusOnlineTTL:
			item.Status = StatusOnline
		case age <= statusStaleTTL:
			item.Status = StatusStale
		default:
			item.Status = StatusOffline
		}

		items = append(items, item)
	}

	statusOrder := func(status string) int {
		switch status {
		case StatusOnline:
			return 0
		case StatusStale:
			return 1
		case StatusOffline:
			return 2
		default:
			return 3
		}
	}

	sort.Slice(items, func(i, j int) bool {
		leftStatusOrder := statusOrder(items[i].Status)
		rightStatusOrder := statusOrder(items[j].Status)

		if leftStatusOrder != rightStatusOrder {
			return leftStatusOrder < rightStatusOrder
		}

		return items[i].GatewayID < items[j].GatewayID
	})

	return items, nil
}
