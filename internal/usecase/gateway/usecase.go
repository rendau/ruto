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
}

const (
	itemTTL         = 10 * time.Minute
	statusOnlineTTL = 20 * time.Second
	statusStaleTTL  = time.Minute
)

func New(sessionSvc SessionServiceI, cache CacheI) *Usecase {
	return &Usecase{
		sessionSvc: sessionSvc,
		cache:      cache,
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
		GatewayID:       gatewayID,
		PodName:         strings.TrimSpace(req.PodName),
		HostName:        strings.TrimSpace(req.HostName),
		SnapshotVersion: strings.TrimSpace(req.SnapshotVersion),
		LastApplyAtUnix: req.LastApplyAtUnix,
		StartedAtUnix:   req.StartedAtUnix,
		LastError:       strings.TrimSpace(req.LastError),
		LastSeenAtUnix:  time.Now().Unix(),
	}

	if err := u.cache.SetJsonObj(gatewayID, item, itemTTL); err != nil {
		return fmt.Errorf("cache.SetJsonObj: %w", err)
	}

	return nil
}

func (u *Usecase) List(ctx context.Context) ([]*Item, error) {
	extractedSession := u.sessionSvc.FromContext(ctx)
	if extractedSession.Id == 0 {
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

	sort.Slice(items, func(i, j int) bool {
		return items[i].GatewayID < items[j].GatewayID
	})

	return items, nil
}
