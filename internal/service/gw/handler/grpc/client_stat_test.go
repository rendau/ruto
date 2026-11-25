package grpc

import (
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"

	rateLimitModel "github.com/rendau/ruto/internal/domain/ratelimit/model"
	rootModel "github.com/rendau/ruto/internal/domain/root/model"
	"github.com/rendau/ruto/internal/infra/clientip"
	"github.com/rendau/ruto/internal/service/gw/service/blocklist"
	"github.com/rendau/ruto/internal/service/gw/service/clientstat"
	"github.com/rendau/ruto/internal/service/gw/service/ratelimit"
)

func TestClientStat_CountsEveryCall(t *testing.T) {
	backendAddr, backendStop := runBackendTestService(t)
	defer backendStop()

	snapshot := buildSnapshotForBackend(t, backendAddr)
	snapshot.TrustedClients = []rootModel.RootTrustedClient{{Name: "office", Ip: "198.51.100.0/24"}}
	snapshot.Apps[0].Endpoints[0].RateLimit = rateLimitModel.RateLimit{Rules: []*rateLimitModel.Rule{
		{Name: "unary", Key: "ip", Requests: 1, Period: "1m"},
	}}
	require.NoError(t, snapshot.Normalize())
	snapshot.InheritDown()

	clientIP := clientip.New([]string{"127.0.0.0/8"}, nil)
	blocks := blocklist.New(clientIP)
	blocks.Set("v1", []blocklist.Rule{{Ip: "203.0.113.0/24"}})
	clients := clientstat.NewCollector(clientstat.MaxClients)
	env := newBlocklistEnv(t, snapshot, blocks,
		ratelimit.NewService(ratelimit.New(1000), clientIP),
		clientstat.New(clients, clientIP),
	)

	const blocked, limited = "203.0.113.7", "192.0.2.1"
	require.Equal(t, codes.PermissionDenied, env.unary(t, "backend-test", blocked))
	require.Equal(t, codes.PermissionDenied, env.unary(t, "unknown-app", blocked))
	require.Equal(t, codes.PermissionDenied, env.listServices(t, blocked))

	require.Equal(t, codes.OK, env.unary(t, "backend-test", limited))
	require.Equal(t, codes.ResourceExhausted, env.unary(t, "backend-test", limited))
	require.Equal(t, codes.NotFound, env.unary(t, "unknown-app", limited))
	require.Equal(t, codes.OK, env.listServices(t, limited))

	for _, ip := range []string{"198.51.100.1", "198.51.100.2", "198.51.100.1"} {
		require.Equal(t, codes.OK, env.unary(t, "backend-test", ip))
	}

	got := lo.KeyBy(clients.Drain(), func(v *clientstat.Item) string { return v.Ip })
	assert.Equal(t, &clientstat.Item{Ip: blocked, Requests: 3, Blocked: 3}, got[blocked])
	assert.Equal(t, &clientstat.Item{Ip: limited, Requests: 4, Limited: 1, LastRule: "unary"}, got[limited])
	assert.Equal(t, &clientstat.Item{Ip: "198.51.100.0/24", Trusted: "office", Requests: 3}, got["198.51.100.0/24"])
	assert.Len(t, got, 3)
}
