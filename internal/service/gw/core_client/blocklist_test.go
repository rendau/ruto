package core_client

import (
	"context"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	rootModel "github.com/rendau/ruto/internal/domain/root/model"
	"github.com/rendau/ruto/internal/infra/clientip"
	"github.com/rendau/ruto/internal/service/gw/service/blocklist"
	"github.com/rendau/ruto/pkg/proto/ruto_v1"
)

// fakeCore is core as the gateway sees it: the snapshot is always empty, the
// block list is whatever the test sets.
type fakeCore struct {
	mu        sync.Mutex
	version   string
	rules     []*ruto_v1.BlockedClientRule
	fail      bool
	heartbeat *ruto_v1.GatewayHeartbeatRequest

	snapshotVersionCalls atomic.Int64
	versionCalls         atomic.Int64
	activeCalls          atomic.Int64
	subscribed           atomic.Bool
	notify               chan string
}

func (c *fakeCore) set(version string, fail bool, rules ...*ruto_v1.BlockedClientRule) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.version, c.fail, c.rules = version, fail, rules
}

func (c *fakeCore) lastHeartbeat() *ruto_v1.GatewayHeartbeatRequest {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.heartbeat
}

type snapshotServer struct {
	ruto_v1.UnimplementedSnapshotServer
	*fakeCore
}

type gatewayServer struct {
	ruto_v1.UnimplementedGatewayServer
	*fakeCore
}

type blockedClientServer struct {
	ruto_v1.UnimplementedBlockedClientServer
	*fakeCore
}

func (c *snapshotServer) GetVersion(context.Context, *emptypb.Empty) (*ruto_v1.SnapshotVersion, error) {
	c.snapshotVersionCalls.Add(1)
	return &ruto_v1.SnapshotVersion{}, nil
}

func (c *gatewayServer) Heartbeat(_ context.Context, req *ruto_v1.GatewayHeartbeatRequest) (*emptypb.Empty, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.heartbeat = req
	return &emptypb.Empty{}, nil
}

func (c *gatewayServer) Subscribe(_ *ruto_v1.GatewaySubscribeRequest, stream grpc.ServerStreamingServer[ruto_v1.GatewayNotification]) error {
	c.subscribed.Store(true)
	for {
		select {
		case <-stream.Context().Done():
			return nil
		case typ := <-c.notify:
			if err := stream.Send(&ruto_v1.GatewayNotification{Type: typ}); err != nil {
				return err
			}
		}
	}
}

func (s *blockedClientServer) GetVersion(context.Context, *emptypb.Empty) (*ruto_v1.BlockedClientVersion, error) {
	s.versionCalls.Add(1)
	s.mu.Lock()
	defer s.mu.Unlock()
	return &ruto_v1.BlockedClientVersion{Version: s.version}, nil
}

func (s *blockedClientServer) GetActive(context.Context, *emptypb.Empty) (*ruto_v1.BlockedClientActiveRep, error) {
	s.activeCalls.Add(1)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.fail {
		return nil, status.Error(codes.Unavailable, "db is down")
	}
	return &ruto_v1.BlockedClientActiveRep{Version: s.version, Rules: s.rules}, nil
}

func runFakeCore(t *testing.T, core *fakeCore) string {
	t.Helper()

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	server := grpc.NewServer()
	ruto_v1.RegisterSnapshotServer(server, &snapshotServer{fakeCore: core})
	ruto_v1.RegisterGatewayServer(server, &gatewayServer{fakeCore: core})
	ruto_v1.RegisterBlockedClientServer(server, &blockedClientServer{fakeCore: core})
	go func() { _ = server.Serve(lis) }()
	t.Cleanup(server.Stop)

	return lis.Addr().String()
}

func startClient(t *testing.T, core *fakeCore, poll time.Duration) *blocklist.Service {
	t.Helper()

	ctx, cancel := context.WithCancel(t.Context())
	t.Cleanup(cancel)

	blocks := blocklist.New(clientip.New(nil, nil))
	client, err := New(ctx, runFakeCore(t, core), "", func(*rootModel.Root) error { return nil }, blocks)
	require.NoError(t, err)
	client.blocklistPoll = poll
	client.Start()

	return blocks
}

func rule(ip string) *ruto_v1.BlockedClientRule {
	return &ruto_v1.BlockedClientRule{Ip: ip}
}

func appliedVersion(blocks *blocklist.Service, want string) func() bool {
	return func() bool { return blocks.Version() == want }
}

func TestBlocklist_AppliedOnNotification(t *testing.T) {
	core := &fakeCore{notify: make(chan string)}
	core.set("v1", false, rule("203.0.113.7"), rule("nope"))
	blocks := startClient(t, core, time.Hour)

	// the check at start and the one after the stream opened
	require.Eventually(t, func() bool {
		return blocks.Version() == "v1" && core.subscribed.Load() && core.versionCalls.Load() >= 2
	}, 5*time.Second, 10*time.Millisecond)
	assert.Equal(t, 1, blocks.Len(), "an invalid rule is skipped")
	guard := blocks.Guard(nil, blocklist.ProtocolHTTP)
	assert.True(t, guard.Blocked("203.0.113.7:1", nil))

	core.set("v2", false, rule("198.51.100.0/24"))
	time.Sleep(200 * time.Millisecond)
	assert.Equal(t, "v1", blocks.Version(), "no poll within the hour")

	// a snapshot notification leaves the block list alone
	snapshotCalls, versionCalls := core.snapshotVersionCalls.Load(), core.versionCalls.Load()
	core.notify <- "check_version"
	require.Eventually(t, func() bool { return core.snapshotVersionCalls.Load() > snapshotCalls }, 5*time.Second, 10*time.Millisecond)
	assert.Equal(t, versionCalls, core.versionCalls.Load())
	assert.Equal(t, "v1", blocks.Version())

	core.notify <- "check_blocklist"
	require.Eventually(t, appliedVersion(blocks, "v2"), 5*time.Second, 10*time.Millisecond)
	assert.False(t, guard.Blocked("203.0.113.7:1", nil))
	assert.True(t, guard.Blocked("198.51.100.9:1", nil))
}

func TestBlocklist_AppliedOnPoll(t *testing.T) {
	core := &fakeCore{notify: make(chan string)}
	core.set("v1", false, rule("203.0.113.7"))
	blocks := startClient(t, core, 50*time.Millisecond)

	require.Eventually(t, appliedVersion(blocks, "v1"), 5*time.Second, 10*time.Millisecond)

	core.set("v2", false)
	require.Eventually(t, appliedVersion(blocks, "v2"), 5*time.Second, 10*time.Millisecond)
	assert.Equal(t, 0, blocks.Len())

	// an unchanged version is not fetched again
	activeCalls := core.activeCalls.Load()
	time.Sleep(200 * time.Millisecond)
	assert.Equal(t, activeCalls, core.activeCalls.Load())
}

func TestBlocklist_KeepsLastOnError(t *testing.T) {
	core := &fakeCore{notify: make(chan string)}
	core.set("v1", false, rule("203.0.113.7"))
	blocks := startClient(t, core, 50*time.Millisecond)
	guard := blocks.Guard(nil, blocklist.ProtocolHTTP)

	require.Eventually(t, appliedVersion(blocks, "v1"), 5*time.Second, 10*time.Millisecond)

	core.set("v2", true)
	activeCalls := core.activeCalls.Load()
	require.Eventually(t, func() bool { return core.activeCalls.Load() >= activeCalls+3 }, 5*time.Second, 10*time.Millisecond)
	assert.Equal(t, "v1", blocks.Version())
	assert.True(t, guard.Blocked("203.0.113.7:1", nil))

	core.set("v3", false)
	require.Eventually(t, appliedVersion(blocks, "v3"), 5*time.Second, 10*time.Millisecond)
	assert.False(t, guard.Blocked("203.0.113.7:1", nil))
}

func TestBlocklist_EmptyVersionStillApplied(t *testing.T) {
	core := &fakeCore{notify: make(chan string)}
	blocks := startClient(t, core, 50*time.Millisecond)

	require.Eventually(t, blocks.Applied, 5*time.Second, 10*time.Millisecond)
	assert.Empty(t, blocks.Version())
}

func TestBlocklist_HeartbeatCarriesVersion(t *testing.T) {
	core := &fakeCore{notify: make(chan string)}
	core.set("v1", false, rule("203.0.113.7"))
	blocks := startClient(t, core, 50*time.Millisecond)

	require.Eventually(t, appliedVersion(blocks, "v1"), 5*time.Second, 10*time.Millisecond)
	// applying triggers a heartbeat at once, not after HeartbeatInterval
	require.Eventually(t, func() bool {
		hb := core.lastHeartbeat()
		return hb != nil && hb.GetBlocklistVersion() == "v1"
	}, 5*time.Second, 10*time.Millisecond)
}
