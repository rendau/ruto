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
	"google.golang.org/grpc/status"

	rateLimitModel "github.com/rendau/ruto/internal/domain/ratelimit/model"
	"github.com/rendau/ruto/internal/infra/clientip"
	"github.com/rendau/ruto/internal/service/gw/service/ratelimit"
)

func TestRateLimit_ResourceExhausted(t *testing.T) {
	backendAddr, backendStop := runBackendTestService(t)
	defer backendStop()

	snapshot := buildSnapshotForBackend(t, backendAddr)
	snapshot.Apps[0].Endpoints[0].RateLimit = rateLimitModel.RateLimit{Rules: []*rateLimitModel.Rule{
		{Name: "unary", Key: "token", Requests: 2, Period: "1m"},
	}}
	require.NoError(t, snapshot.Normalize())
	snapshot.InheritDown()

	limits := ratelimit.NewService(ratelimit.New(1000), clientip.New(nil, nil))
	svc, err := New(snapshot, NewConns(), limits, nil, nil, nil)
	require.NoError(t, err)

	gatewayAddr, gatewayStop := runGatewayProxyServer(t, svc)
	defer gatewayStop()

	conn, err := gogrpc.NewClient(gatewayAddr, gogrpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	client := grpcTesting.NewTestServiceClient(conn)

	call := func(token string) (codes.Code, metadata.MD) {
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()
		ctx = metadata.AppendToOutgoingContext(ctx, metadataHeaderAppName, "backend-test", "authorization", token)

		var trailer metadata.MD
		_, callErr := client.UnaryCall(ctx, &grpcTesting.SimpleRequest{
			Payload: &grpcTesting.Payload{Body: []byte("ping")},
		}, gogrpc.Trailer(&trailer))
		return status.Code(callErr), trailer
	}

	for range 2 {
		code, _ := call("Bearer aaa")
		require.Equal(t, codes.OK, code)
	}

	code, trailer := call("Bearer aaa")
	require.Equal(t, codes.ResourceExhausted, code)
	require.Len(t, trailer.Get(metadataHeaderRetryAfter), 1)
	assert.NotEqual(t, "0", trailer.Get(metadataHeaderRetryAfter)[0])

	// the same token spelled otherwise shares the counter
	code, _ = call("aaa")
	assert.Equal(t, codes.ResourceExhausted, code)
	code, _ = call("bearer  aaa")
	assert.Equal(t, codes.ResourceExhausted, code)

	// another token has its own counter
	code, _ = call("Bearer bbb")
	assert.Equal(t, codes.OK, code)
}

func TestRateLimitError_IsOwnReject(t *testing.T) {
	err := error(&rateLimitError{retryAfter: 3})

	assert.ErrorIs(t, err, errOwnReject)
	assert.Equal(t, codes.ResourceExhausted, status.Code(err))
	assert.Equal(t, "rate limit exceeded, retry after 3s", status.Convert(err).Message())
}
