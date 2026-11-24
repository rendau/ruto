package core_client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"runtime"
	"runtime/metrics"
	"strings"
	"sync"
	"time"

	"encoding/json"

	"github.com/samber/lo"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/protobuf/types/known/emptypb"

	rootModel "github.com/rendau/ruto/internal/domain/root/model"
	"github.com/rendau/ruto/internal/service/gw/service/blocklist"
	"github.com/rendau/ruto/internal/service/gw/service/seen"
	"github.com/rendau/ruto/pkg/proto/ruto_v1"
)

const (
	// HeartbeatInterval is how often the gateway reports its state to core.
	// Snapshot version checks are NOT periodic — they are driven by push
	// notifications over the Subscribe stream plus an initial check on
	// (re)connect, so there is no separate polling interval.
	HeartbeatInterval = 30 * time.Second

	// heartbeatTimeout bounds a single Heartbeat RPC. It is generous on purpose:
	// the heartbeat is pure telemetry, so a slow core (or its cache backend) must
	// not produce warning noise as long as the report eventually lands well
	// within the interval.
	heartbeatTimeout = 15 * time.Second

	// subscribeReconnectDelay is how long to wait before re-opening the
	// notification stream to core after it drops.
	subscribeReconnectDelay = 2 * time.Second

	// BlocklistPollInterval is how often the gateway checks the block list
	// version on its own. Unlike the snapshot the block list is polled: it is
	// changed by an agent during an attack, and a missed "check_blocklist"
	// must not leave a block unapplied for long.
	BlocklistPollInterval = 10 * time.Second

	rpcTimeout = 5 * time.Second

	notificationCheckBlocklist = "check_blocklist"
)

type configCallback func(*rootModel.Root) error

// blocklistI is where the block list fetched from core goes, see
// gw/service/blocklist.
type blocklistI interface {
	Set(version string, rules []blocklist.Rule)
	Version() string
	Applied() bool
	Len() int
}

type Service struct {
	globalCtx context.Context
	onConfig  configCallback

	conn          *grpc.ClientConn
	client        ruto_v1.SnapshotClient
	gatewayClient ruto_v1.GatewayClient
	blockedClient ruto_v1.BlockedClientClient
	identity      *identity
	startedAtUnix int64

	// mu guards the heartbeat-reported state below, which refreshWorker writes
	// and heartbeatWorker reads from a different goroutine.
	mu             sync.Mutex
	currentVersion string
	lastApplyAt    int64
	lastError      string

	// triggerCh requests a refresh (version check + apply). It is
	// buffered/coalescing so bursts collapse to a single pending refresh.
	// The push-notification stream feeds it (plus one initial check at
	// startup), and a single refresh goroutine drains it — that single
	// consumer is what guarantees version checks and snapshot applies never
	// run in parallel.
	triggerCh chan struct{}

	// heartbeatTrigger forces an immediate heartbeat (outside the periodic
	// tick), used right after a snapshot is applied so core learns about the
	// new version without waiting for the next interval. Buffered/coalescing.
	heartbeatTrigger chan struct{}

	// blocks is the block list; blocklistWorker alone refreshes it, on a tick
	// and on blocklistTrigger (a "check_blocklist" notification or a
	// reconnect of the stream). Buffered/coalescing like triggerCh.
	blocks           blocklistI
	blocklistTrigger chan struct{}
	blocklistPoll    time.Duration
	// blocklistFailing is touched by blocklistWorker only: an unreachable
	// core is reported once, not on every poll.
	blocklistFailing bool
}

func New(
	globalCtx context.Context,
	address string,
	onConfig configCallback,
	blocks blocklistI,
) (*Service, error) {
	var err error

	service := &Service{
		globalCtx:        globalCtx,
		onConfig:         onConfig,
		identity:         newIdentity(),
		startedAtUnix:    time.Now().Unix(),
		triggerCh:        make(chan struct{}, 1),
		heartbeatTrigger: make(chan struct{}, 1),
		blocks:           blocks,
		blocklistTrigger: make(chan struct{}, 1),
		blocklistPoll:    BlocklistPollInterval,
	}

	if address != "" {
		service.conn, err = grpc.NewClient(
			"dns:///"+address,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithKeepaliveParams(
				keepalive.ClientParameters{
					Time:                10 * time.Second,
					Timeout:             2 * time.Second,
					PermitWithoutStream: true,
				},
			),
		)
		if err != nil {
			return nil, fmt.Errorf("grpc.NewClient: %w", err)
		}
		service.client = ruto_v1.NewSnapshotClient(service.conn)
		service.gatewayClient = ruto_v1.NewGatewayClient(service.conn)
		if blocks != nil {
			service.blockedClient = ruto_v1.NewBlockedClientClient(service.conn)
		}
	}

	return service, nil
}

func (s *Service) Start() {
	if s.client != nil {
		go s.refreshWorker()
	}
	if s.gatewayClient != nil {
		go s.heartbeatWorker()
		go s.subscribeWorker()
	}
	if s.blockedClient != nil {
		go s.blocklistWorker()
	}
}

// refreshWorker is the single goroutine that runs refresh(). Because it is the
// only caller, version checks and snapshot applies are inherently serialized.
// It does one initial check at startup, then only reacts to triggers from the
// push-notification stream — there is no periodic polling.
func (s *Service) refreshWorker() {
	select {
	case <-s.globalCtx.Done():
		return
	case <-time.After(time.Second):
	}

	for {
		s.refresh()

		select {
		case <-s.globalCtx.Done():
			return
		case <-s.triggerCh:
		}
	}
}

// heartbeatWorker reports the gateway's state to core every HeartbeatInterval,
// and also immediately whenever heartbeatTrigger fires (e.g. right after a
// snapshot is applied). All heartbeats are sent from this single goroutine, so
// the Heartbeat RPC is never invoked concurrently.
func (s *Service) heartbeatWorker() {
	ticker := time.NewTicker(HeartbeatInterval)
	defer ticker.Stop()

	for {
		if err := s.sendHeartbeat(); err != nil {
			slog.Warn("core-client: heartbeat failed", "error", err)
		}

		select {
		case <-s.globalCtx.Done():
			return
		case <-ticker.C:
		case <-s.heartbeatTrigger:
		}
	}
}

// triggerHeartbeat requests an immediate heartbeat without blocking. Redundant
// triggers are dropped because heartbeatTrigger is buffered to one.
func (s *Service) triggerHeartbeat() {
	select {
	case s.heartbeatTrigger <- struct{}{}:
	default:
	}
}

// trigger requests an early refresh without blocking. Redundant triggers are
// dropped because triggerCh is buffered to one.
func (s *Service) trigger() {
	select {
	case s.triggerCh <- struct{}{}:
	default:
	}
}

// triggerBlocklist requests an early block list check without blocking.
func (s *Service) triggerBlocklist() {
	select {
	case s.blocklistTrigger <- struct{}{}:
	default:
	}
}

// subscribeWorker keeps the notification stream to core open for the whole
// lifetime of the gateway, reconnecting forever with a small delay whenever the
// stream drops (core restart, network blip, etc.).
func (s *Service) subscribeWorker() {
	for s.globalCtx.Err() == nil {
		if err := s.subscribeOnce(); err != nil && s.globalCtx.Err() == nil {
			if !errors.Is(err, io.EOF) {
				slog.Warn("core-client: notification stream ended", "error", err)
			}
		}

		select {
		case <-s.globalCtx.Done():
			return
		case <-time.After(subscribeReconnectDelay):
		}
	}
}

// subscribeOnce opens one Subscribe stream and drains notifications until it
// errors. Each notification just enqueues a trigger: "check_blocklist" of the
// block list, anything else of the snapshot.
func (s *Service) subscribeOnce() error {
	stream, err := s.gatewayClient.Subscribe(s.globalCtx, &ruto_v1.GatewaySubscribeRequest{
		GatewayId: s.identity.GatewayID,
	})
	if err != nil {
		return fmt.Errorf("gatewayClient.Subscribe: %w", err)
	}

	// Blocks added while the stream was down were not pushed to us.
	s.triggerBlocklist()

	for {
		notification, recvErr := stream.Recv()
		if recvErr != nil {
			if s.globalCtx.Err() != nil {
				return nil
			}
			return fmt.Errorf("stream.Recv: %w", recvErr)
		}

		if notification.GetType() == notificationCheckBlocklist {
			s.triggerBlocklist()
			continue
		}
		s.trigger()
	}
}

func (s *Service) refresh() {
	serverVersion, err := s.fetchVersion()
	if err != nil {
		s.recordError(err.Error())
		slog.Error("config-client: fetchVersion failed", "error", err)
		return
	}
	if serverVersion == "" || serverVersion == s.readVersion() {
		return
	}

	err = s.fetchAndApplyConfig()
	if err != nil {
		s.recordError(err.Error())
		slog.Error("config-client: fetchAndApplyConfig failed", "error", err, "version", serverVersion)
		return
	}

	slog.Info("config-client: config applied", "version", serverVersion)

	s.recordApplied(serverVersion)

	// Let core know about the new version right away instead of waiting for the
	// next periodic heartbeat.
	s.triggerHeartbeat()
}

func (s *Service) readVersion() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.currentVersion
}

func (s *Service) recordApplied(version string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.currentVersion = version
	s.lastApplyAt = time.Now().Unix()
	s.lastError = ""
}

func (s *Service) recordError(msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastError = msg
}

func (s *Service) fetchVersion() (string, error) {
	ctx, cancel := context.WithTimeout(s.globalCtx, rpcTimeout)
	defer cancel()

	versionRep, err := s.client.GetVersion(ctx, &emptypb.Empty{})
	if err != nil {
		if s.globalCtx.Err() == nil {
			return "", fmt.Errorf("client.GetVersion: %w", err)
		}
		return "", nil
	}

	return versionRep.GetVersion(), nil
}

func (s *Service) fetchAndApplyConfig() error {
	ctx, cancel := context.WithTimeout(s.globalCtx, rpcTimeout)
	defer cancel()

	configRep, err := s.client.Get(ctx, &emptypb.Empty{})
	if err != nil {
		if s.globalCtx.Err() == nil {
			return fmt.Errorf("client.Get: %w", err)
		}
		return nil
	}

	conf, err := decodeConfig(configRep)
	if err != nil {
		return fmt.Errorf("decodeConfig: %w", err)
	}

	err = s.onConfig(conf)
	if err != nil {
		return fmt.Errorf("onConfig: %w", err)
	}

	return nil
}

func (s *Service) sendHeartbeat() error {
	if s.gatewayClient == nil || s.identity == nil {
		return nil
	}

	s.mu.Lock()
	currentVersion := s.currentVersion
	lastApplyAt := s.lastApplyAt
	lastError := strings.TrimSpace(s.lastError)
	s.mu.Unlock()

	if len(lastError) > 512 {
		lastError = lastError[:512]
	}

	seenItems := seen.Ins().Drain()
	seenOrigins := seen.OriginsIns().Drain()

	ctx, cancel := context.WithTimeout(s.globalCtx, heartbeatTimeout)
	defer cancel()

	blocklistVersion := ""
	if s.blocks != nil {
		blocklistVersion = s.blocks.Version()
	}

	_, err := s.gatewayClient.Heartbeat(ctx, &ruto_v1.GatewayHeartbeatRequest{
		GatewayId:        s.identity.GatewayID,
		HostName:         s.identity.HostName,
		SnapshotVersion:  currentVersion,
		LastApplyAtUnix:  lastApplyAt,
		StartedAtUnix:    s.startedAtUnix,
		LastError:        lastError,
		MemoryAllocBytes: heapAllocBytes(),
		GoroutinesCount:  uint32(runtime.NumGoroutine()),
		SeenPaths:        lo.Map(seenItems, encodeSeenPath),
		SeenOrigins:      lo.Map(seenOrigins, encodeSeenOrigin),
		BlocklistVersion: blocklistVersion,
	})
	if err != nil {
		// Not delivered: keep the counters for the next heartbeat.
		seen.Ins().Restore(seenItems)
		seen.OriginsIns().Restore(seenOrigins)
		if s.globalCtx.Err() == nil {
			return fmt.Errorf("gatewayClient.Heartbeat: %w", err)
		}
	}

	return nil
}

func encodeSeenPath(v *seen.Item, _ int) *ruto_v1.GatewaySeenPath {
	return &ruto_v1.GatewaySeenPath{
		AppId:          v.AppId,
		EndpointId:     v.EndpointId,
		Method:         v.Method,
		Path:           v.Path,
		Hits:           v.Hits,
		HitsNotFound:   v.HitsNotFound,
		LastSeenAtUnix: v.LastSeenAt.Unix(),
		Sample:         v.Sample,
	}
}

func encodeSeenOrigin(v *seen.OriginItem, _ int) *ruto_v1.GatewaySeenOrigin {
	return &ruto_v1.GatewaySeenOrigin{
		Origin:         v.Origin,
		Hits:           v.Hits,
		LastSeenAtUnix: v.LastSeenAt.Unix(),
	}
}

// heapAllocBytes reports live heap bytes, the equivalent of MemStats.Alloc.
// runtime/metrics is used instead of runtime.ReadMemStats because the latter
// stops the world on every call; the tradeoff is that per-P allocator caches
// may not be flushed yet, so the value can lag slightly — irrelevant for
// telemetry.
func heapAllocBytes() uint64 {
	samples := []metrics.Sample{{Name: "/memory/classes/heap/objects:bytes"}}
	metrics.Read(samples)

	if samples[0].Value.Kind() != metrics.KindUint64 {
		return 0
	}

	return samples[0].Value.Uint64()
}

func decodeConfig(rep *ruto_v1.SnapshotResponse) (*rootModel.Root, error) {
	if rep == nil || rep.Data == nil {
		return rootModel.NewEmpty(), nil
	}

	body, err := json.Marshal(rep.Data.AsMap())
	if err != nil {
		return nil, fmt.Errorf("json.Marshal: %w", err)
	}

	result := new(rootModel.Root{})
	if err = json.Unmarshal(body, result); err != nil {
		return nil, fmt.Errorf("json.Unmarshal: %w", err)
	}

	return result, nil
}
