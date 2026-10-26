// Package system serves ruto's state to the system port (the pulse manifest and
// the routes for pulse's own ruto integration). Nothing here checks a login:
// the system port is reachable only inside the cluster, so every result must be
// free of secrets.
package system

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/samber/lo"

	appModel "github.com/rendau/ruto/internal/domain/app/model"
	changeRequestModel "github.com/rendau/ruto/internal/domain/changerequest/model"
	endpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
	rootModel "github.com/rendau/ruto/internal/domain/root/model"
	usecaseGateway "github.com/rendau/ruto/internal/usecase/gateway"
)

// LagAfter — a publish reaches an online gateway within seconds (core pushes a
// notification), so a gateway still on the old version after this is failing
// to apply it.
const LagAfter = 2 * time.Minute

type Usecase struct {
	snapshotSvc      SnapshotServiceI
	snapshotUc       SnapshotUsecaseI
	gatewayUc        GatewayUsecaseI
	gatewaysPool     GatewaysPoolI
	changeRequestSvc ChangeRequestServiceI

	// when core first saw the current published version: the start of the lag
	// window (after a core restart — the restart time)
	mu                 sync.Mutex
	publishedVersion   string
	publishedVersionAt time.Time
}

func New(
	snapshotSvc SnapshotServiceI,
	snapshotUc SnapshotUsecaseI,
	gatewayUc GatewayUsecaseI,
	gatewaysPool GatewaysPoolI,
	changeRequestSvc ChangeRequestServiceI,
) *Usecase {
	return &Usecase{
		snapshotSvc:      snapshotSvc,
		snapshotUc:       snapshotUc,
		gatewayUc:        gatewayUc,
		gatewaysPool:     gatewaysPool,
		changeRequestSvc: changeRequestSvc,
	}
}

// Gateways — every gateway core knows of: from heartbeats (kept for a while
// after a gateway stops) and from open notification streams. A connected
// gateway without fresh heartbeats is Silent: alive but not reporting.
func (u *Usecase) Gateways(ctx context.Context) (*Gateways, error) {
	published, err := u.snapshotSvc.GetVersion(ctx)
	if err != nil {
		return nil, fmt.Errorf("snapshotSvc.GetVersion: %w", err)
	}
	lagging := time.Since(u.seenPublished(published)) > LagAfter

	items, err := u.gatewayUc.ListAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("gatewayUc.ListAll: %w", err)
	}
	connected := lo.SliceToMap(u.gatewaysPool.ConnectedIDs(), func(id string) (string, bool) { return id, true })

	result := &Gateways{PublishedVersion: published}
	for _, item := range items {
		gw := &Gateway{
			HostName:    lo.CoalesceOrEmpty(item.HostName, item.GatewayID),
			Status:      item.Status,
			Connected:   connected[item.GatewayID],
			Version:     item.SnapshotVersion,
			UpToDate:    item.SnapshotVersion == published,
			LastApplyAt: unixTime(item.LastApplyAtUnix),
			LastSeenAt:  unixTime(item.LastSeenAtUnix),
			LastError:   item.LastError,
		}
		gw.Lagging = lagging && !gw.UpToDate && gw.Status == usecaseGateway.StatusOnline
		gw.Silent = gw.Connected && gw.Status == usecaseGateway.StatusOffline
		result.Gateways = append(result.Gateways, gw)
		delete(connected, item.GatewayID)
	}
	// connected, but no heartbeat at all for longer than core keeps one
	for id := range connected {
		result.Gateways = append(result.Gateways, &Gateway{
			HostName:  id,
			Status:    usecaseGateway.StatusOffline,
			Connected: true,
			Silent:    true,
		})
	}

	return result, nil
}

func (u *Usecase) seenPublished(version string) time.Time {
	u.mu.Lock()
	defer u.mu.Unlock()

	if version != u.publishedVersion {
		u.publishedVersion = version
		u.publishedVersionAt = time.Now()
	}

	return u.publishedVersionAt
}

// HasUnpublishedChanges builds the snapshot from the current configuration and
// compares it with the published one.
func (u *Usecase) HasUnpublishedChanges(ctx context.Context) (bool, error) {
	published, err := u.snapshotSvc.GetVersion(ctx)
	if err != nil {
		return false, fmt.Errorf("snapshotSvc.GetVersion: %w", err)
	}
	current, err := u.snapshotUc.CurrentVersion(ctx)
	if err != nil {
		return false, fmt.Errorf("snapshotUc.CurrentVersion: %w", err)
	}

	return current != published, nil
}

func (u *Usecase) PendingChangeRequests(ctx context.Context, stuckAfter time.Duration) (*changeRequestModel.PendingStats, error) {
	result, err := u.changeRequestSvc.PendingStats(ctx, stuckAfter)
	if err != nil {
		return nil, fmt.Errorf("changeRequestSvc.PendingStats: %w", err)
	}
	return result, nil
}

func (u *Usecase) RoutesVersion(ctx context.Context) (string, error) {
	version, err := u.snapshotSvc.GetVersion(ctx)
	if err != nil {
		return "", fmt.Errorf("snapshotSvc.GetVersion: %w", err)
	}
	return version, nil
}

// Routes — the published snapshot (what the gateways run), reduced to Routes.
func (u *Usecase) Routes(ctx context.Context) (*Routes, error) {
	snapshot, err := u.snapshotSvc.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("snapshotSvc.Get: %w", err)
	}

	root := &rootModel.Root{}
	if len(snapshot.Data) > 0 {
		if err = json.Unmarshal(snapshot.Data, root); err != nil {
			return nil, fmt.Errorf("json.Unmarshal: %w", err)
		}
	}

	return &Routes{
		Version: snapshot.Hash,
		BaseUrl: root.BaseUrl,
		Apps:    lo.Map(root.Apps, encodeRouteApp),
	}, nil
}

func encodeRouteApp(v *appModel.App, _ int) *RouteApp {
	return &RouteApp{
		Id:                 v.Id,
		Name:               v.Name,
		Active:             v.Active,
		PathPrefix:         v.PathPrefix,
		ExcludeFromMetrics: v.ExcludeFromMetrics,
		BackendUrl:         safeAddress(v.Backend.Url),
		GrpcUrl:            safeAddress(v.Backend.GrpcUrl),
		Endpoints:          lo.Map(v.Endpoints, encodeRouteEndpoint),
	}
}

func encodeRouteEndpoint(v *endpointModel.Endpoint, _ int) *RouteEndpoint {
	return &RouteEndpoint{
		Id:       v.Id,
		Active:   v.Active,
		Type:     string(v.Type),
		Method:   v.Http.Method,
		Path:     v.Http.Path,
		GrpcPath: v.Grpc.Path,
	}
}

// safeAddress drops what may carry a credential: user:password@ and the query
// (?key=…). An address that does not parse is dropped whole.
func safeAddress(v string) string {
	if v == "" {
		return ""
	}

	if strings.Contains(v, "://") {
		parsed, err := url.Parse(v)
		if err != nil {
			return ""
		}
		parsed.User = nil
		parsed.RawQuery = ""
		parsed.ForceQuery = false
		parsed.Fragment = ""
		return parsed.String()
	}

	// host:port (grpc)
	if i := strings.LastIndex(v, "@"); i >= 0 {
		v = v[i+1:]
	}
	if i := strings.IndexAny(v, "?#"); i >= 0 {
		v = v[:i]
	}
	return v
}

func unixTime(v int64) *time.Time {
	if v <= 0 {
		return nil
	}
	return new(time.Unix(v, 0))
}
