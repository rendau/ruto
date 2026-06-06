package gateway

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/rendau/ruto/internal/errs"
)

type Usecase struct {
	sessionSvc SessionServiceI
	cache      CacheI
	gateways   GatewaysI
}

const (
	itemTTL         = 10 * time.Minute
	statusOnlineTTL = 20 * time.Second
	statusStaleTTL  = time.Minute
)

func New(sessionSvc SessionServiceI, cache CacheI, gateways GatewaysI) *Usecase {
	return &Usecase{
		sessionSvc: sessionSvc,
		cache:      cache,
		gateways:   gateways,
	}
}

// Subscribe registers a connected gateway in the in-memory pool and blocks for
// the lifetime of its stream. Whenever core wants the gateway to re-check the
// snapshot version, the registered trigger fires and `send` pushes a
// notification down the stream. The call returns when the stream context is
// done (gateway disconnected) or `send` fails, removing the gateway from the
// pool via the deferred Unregister.
func (u *Usecase) Subscribe(ctx context.Context, gatewayID string, send func() error) error {
	gatewayID = strings.TrimSpace(gatewayID)

	// buffered+coalescing: redundant triggers collapse into a single pending
	// notification, so a burst of deploys still results in one re-check.
	notifyCh := make(chan struct{}, 1)
	trigger := func() {
		select {
		case notifyCh <- struct{}{}:
		default:
		}
	}

	id := u.gateways.Register(gatewayID, trigger)
	defer u.gateways.Unregister(id)

	// Ask the gateway to sync right after (re)connect: while it was
	// disconnected, the snapshot version may have changed.
	trigger()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-notifyCh:
			if err := send(); err != nil {
				return fmt.Errorf("send: %w", err)
			}
		}
	}
}

func (u *Usecase) Heartbeat(_ context.Context, req *Heartbeat) error {
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

	return nil
}

func (u *Usecase) List(ctx context.Context) ([]*Item, error) {
	if !u.sessionSvc.CtxIsAuthorized(ctx) {
		return nil, errs.NotAuthorized
	}

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
			item.Status = "online"
		case age <= statusStaleTTL:
			item.Status = "stale"
		default:
			item.Status = "offline"
		}

		items = append(items, item)
	}

	statusOrder := func(status string) int {
		switch status {
		case "online":
			return 0
		case "stale":
			return 1
		case "offline":
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
