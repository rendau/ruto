package grpc

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gogrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	grpcTesting "google.golang.org/grpc/interop/grpc_testing"
	"google.golang.org/grpc/metadata"
	reflectionv1 "google.golang.org/grpc/reflection/grpc_reflection_v1"
	"google.golang.org/grpc/status"

	rateLimitModel "github.com/rendau/ruto/internal/domain/ratelimit/model"
	rootModel "github.com/rendau/ruto/internal/domain/root/model"
	"github.com/rendau/ruto/internal/infra/clientip"
	"github.com/rendau/ruto/internal/service/gw/service/blocklist"
	"github.com/rendau/ruto/internal/service/gw/service/clientstat"
	"github.com/rendau/ruto/internal/service/gw/service/ratelimit"
)

type blocklistEnv struct {
	client     grpcTesting.TestServiceClient
	reflection reflectionv1.ServerReflectionClient
}

// newBlocklistEnv runs the gateway for snapshot; the test client connects
// from 127.0.0.1, which is a trusted proxy here, so x-forwarded-for picks the
// client address.
func newBlocklistEnv(
	t *testing.T,
	snapshot *rootModel.Root,
	blocks *blocklist.Service,
	limits *ratelimit.Service,
	stats *clientstat.Service,
) *blocklistEnv {
	t.Helper()

	svc, err := New(snapshot, NewConns(), limits, blocks, stats, nil)
	require.NoError(t, err)

	gatewayAddr, gatewayStop := runGatewayProxyServer(t, svc)
	t.Cleanup(gatewayStop)

	conn, err := gogrpc.NewClient(gatewayAddr, gogrpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	return &blocklistEnv{
		client:     grpcTesting.NewTestServiceClient(conn),
		reflection: reflectionv1.NewServerReflectionClient(conn),
	}
}

func blocklistCtx(t *testing.T, appName, clientIP string) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	pairs := []string{"x-forwarded-for", clientIP}
	if appName != "" {
		pairs = append(pairs, metadataHeaderAppName, appName)
	}
	return metadata.AppendToOutgoingContext(ctx, pairs...), cancel
}

func (e *blocklistEnv) unary(t *testing.T, appName, clientIP string) codes.Code {
	ctx, cancel := blocklistCtx(t, appName, clientIP)
	defer cancel()

	_, err := e.client.UnaryCall(ctx, &grpcTesting.SimpleRequest{Payload: &grpcTesting.Payload{Body: []byte("ping")}})
	return status.Code(err)
}

func (e *blocklistEnv) listServices(t *testing.T, clientIP string) codes.Code {
	ctx, cancel := blocklistCtx(t, "backend-test", clientIP)
	defer cancel()

	stream, err := e.reflection.ServerReflectionInfo(ctx)
	require.NoError(t, err)
	require.NoError(t, stream.Send(&reflectionv1.ServerReflectionRequest{
		MessageRequest: &reflectionv1.ServerReflectionRequest_ListServices{},
	}))
	require.NoError(t, stream.CloseSend())

	_, err = stream.Recv()
	return status.Code(err)
}

func TestBlocklist_PermissionDenied(t *testing.T) {
	backendAddr, backendStop := runBackendTestService(t)
	defer backendStop()

	blocks := blocklist.New(clientip.New([]string{"127.0.0.0/8"}, nil))
	blocks.Set("v1", []blocklist.Rule{{Ip: "203.0.113.0/24"}})
	env := newBlocklistEnv(t, buildSnapshotForBackend(t, backendAddr), blocks, nil, nil)

	const blocked, other = "203.0.113.7", "198.51.100.1"

	assert.Equal(t, codes.PermissionDenied, env.unary(t, "backend-test", blocked))
	assert.Equal(t, codes.PermissionDenied, env.unary(t, "unknown-app", blocked), "unknown route")
	assert.Equal(t, codes.PermissionDenied, env.unary(t, "", blocked), "no app metadata")
	assert.Equal(t, codes.PermissionDenied, env.listServices(t, blocked), "server reflection")

	assert.Equal(t, codes.OK, env.unary(t, "backend-test", other))
	assert.Equal(t, codes.NotFound, env.unary(t, "unknown-app", other))
	assert.Equal(t, codes.OK, env.listServices(t, other))

	// a block lifted in core lets the client in without a new snapshot
	blocks.Set("v2", nil)
	assert.Equal(t, codes.OK, env.unary(t, "backend-test", blocked))
	assert.Equal(t, codes.OK, env.listServices(t, blocked))
}

func TestBlocklist_TrustedClientNotBlockedNorLimited(t *testing.T) {
	backendAddr, backendStop := runBackendTestService(t)
	defer backendStop()

	snapshot := buildSnapshotForBackend(t, backendAddr)
	snapshot.TrustedClients = []rootModel.RootTrustedClient{{Name: "office", Ip: "203.0.113.7"}}
	snapshot.Apps[0].Endpoints[0].RateLimit = rateLimitModel.RateLimit{Rules: []*rateLimitModel.Rule{
		{Key: "ip", Requests: 1, Period: "1m"},
	}}
	require.NoError(t, snapshot.Normalize())
	snapshot.InheritDown()

	clientIP := clientip.New([]string{"127.0.0.0/8"}, nil)
	blocks := blocklist.New(clientIP)
	blocks.Set("v1", []blocklist.Rule{{Ip: "203.0.113.0/24"}})
	env := newBlocklistEnv(t, snapshot, blocks, ratelimit.NewService(ratelimit.New(1000), clientIP), nil)

	for range 3 {
		require.Equal(t, codes.OK, env.unary(t, "backend-test", "203.0.113.7"))
	}
	assert.Equal(t, codes.OK, env.listServices(t, "203.0.113.7"))
	assert.Equal(t, codes.PermissionDenied, env.unary(t, "backend-test", "203.0.113.8"))
	assert.Equal(t, codes.OK, env.unary(t, "backend-test", "198.51.100.1"))
	assert.Equal(t, codes.ResourceExhausted, env.unary(t, "backend-test", "198.51.100.1"))
}
