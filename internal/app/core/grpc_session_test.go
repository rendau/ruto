package core

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	sessionModel "github.com/rendau/ruto/internal/domain/session/model"
	sessionService "github.com/rendau/ruto/internal/domain/session/service"
	"github.com/rendau/ruto/internal/errs"
	"github.com/rendau/ruto/internal/infra/clientip"
	"github.com/rendau/ruto/pkg/proto/ruto_v1"
)

const testApiKey = "rtk_0123456789abcdef"

type testApiKeyAuth struct{ calls int }

func (a *testApiKeyAuth) SessionFromKey(_ context.Context, key string) (*sessionModel.Session, error) {
	a.calls++
	if key != testApiKey {
		return nil, errors.New("unknown key")
	}
	return &sessionModel.Session{Id: 1, Admin: true, ApiKeyId: "key-1", ApiKeyName: "pulse"}, nil
}

const testAdminToken = "admin-jwt"

type testAdminAuth struct{ err error }

func (a *testAdminAuth) SessionFromToken(_ context.Context, token string) (*sessionModel.Session, error) {
	if a.err != nil {
		return nil, a.err
	}
	if token != testAdminToken {
		return nil, errs.NotAuthorized
	}
	return &sessionModel.Session{Id: 7, Admin: true}, nil
}

// An api key works on the main API only for BlockedClient List/Create/Remove:
// everywhere else it must not even be looked up.
func TestSessionApiKeyAllowlist(t *testing.T) {
	sessionSvc := sessionService.New("test-secret")
	keyAuth := &testApiKeyAuth{}
	interceptor := GrpcInterceptorSession(sessionSvc, &testAdminAuth{}, keyAuth)

	call := func(method, authorization string) *sessionModel.Session {
		ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", authorization))
		var got *sessionModel.Session
		_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{FullMethod: method}, func(ctx context.Context, _ any) (any, error) {
			got = sessionSvc.FromContext(ctx)
			return nil, nil
		})
		require.NoError(t, err)
		return got
	}

	for _, method := range []string{
		ruto_v1.BlockedClient_List_FullMethodName,
		ruto_v1.BlockedClient_Create_FullMethodName,
		ruto_v1.BlockedClient_Remove_FullMethodName,
	} {
		for _, header := range []string{"Bearer " + testApiKey, "bearer " + testApiKey, testApiKey} {
			got := call(method, header)
			assert.True(t, got.IsAdmin(), method)
			assert.False(t, got.IsHuman(), "a key session is an agent: "+method)
			assert.Equal(t, "key-1", got.ApiKeyId)
		}
		assert.False(t, call(method, "Bearer rtk_wrong").IsAuthorized(), method)
	}

	keyAuth.calls = 0
	for _, method := range []string{
		ruto_v1.Root_Get_FullMethodName,
		ruto_v1.ChangeRequest_Apply_FullMethodName,
		ruto_v1.App_Get_FullMethodName,
		ruto_v1.ApiKey_Create_FullMethodName,
		ruto_v1.Snapshot_Deploy_FullMethodName,
		ruto_v1.BlockedClient_GetActive_FullMethodName,
		ruto_v1.BlockedClient_RemoveAgentBlocks_FullMethodName,
		ruto_v1.ClientStat_Overview_FullMethodName,
		ruto_v1.ClientStat_Top_FullMethodName,
		ruto_v1.ClientStat_Series_FullMethodName,
	} {
		assert.False(t, call(method, "Bearer "+testApiKey).IsAuthorized(), method)
	}
	assert.Zero(t, keyAuth.calls, "the key is not even checked outside the allowlist")

	// the admin login keeps working everywhere
	for _, method := range []string{ruto_v1.Root_Get_FullMethodName, ruto_v1.BlockedClient_Create_FullMethodName} {
		got := call(method, "Bearer "+testAdminToken)
		assert.True(t, got.IsHuman(), method)
		assert.Equal(t, int64(7), got.Id)
	}
	assert.False(t, call(ruto_v1.Root_Get_FullMethodName, "Bearer stale-jwt").IsAuthorized())
}

// A failed token lookup (the database is down) is not a bad token: the call
// fails as unavailable instead of going on unauthenticated, which would sign
// the admin UI out.
func TestSessionAdminLookupFailure(t *testing.T) {
	sessionSvc := sessionService.New("test-secret")
	interceptor := GrpcInterceptorSession(sessionSvc, &testAdminAuth{err: errors.New("db down")}, &testApiKeyAuth{})

	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+testAdminToken))
	called := false
	_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{FullMethod: ruto_v1.Root_Get_FullMethodName}, func(context.Context, any) (any, error) {
		called = true
		return nil, nil
	})
	require.Error(t, err)
	assert.Equal(t, codes.Unavailable, status.Code(err))
	assert.False(t, called)
}

// The client address the grpc-gateway passes on counts only over loopback:
// a direct grpc caller cannot pick the address the sign-in throttle sees.
func TestGrpcClientIP(t *testing.T) {
	call := func(peerAddr string, md metadata.MD) netip.Addr {
		ctx := peer.NewContext(context.Background(), &peer.Peer{Addr: net.TCPAddrFromAddrPort(netip.MustParseAddrPort(peerAddr))})
		if md != nil {
			ctx = metadata.NewIncomingContext(ctx, md)
		}
		var got netip.Addr
		_, err := GrpcInterceptorClientIP()(ctx, nil, &grpc.UnaryServerInfo{}, func(ctx context.Context, _ any) (any, error) {
			got = clientip.FromContext(ctx)
			return nil, nil
		})
		require.NoError(t, err)
		return got
	}

	forwarded := metadata.Pairs(clientIPMetadata, "203.0.113.9")
	assert.Equal(t, netip.MustParseAddr("203.0.113.9"), call("127.0.0.1:50000", forwarded))
	assert.Equal(t, netip.MustParseAddr("203.0.113.9"), call("[::1]:50000", forwarded))
	assert.Equal(t, netip.MustParseAddr("10.1.2.3"), call("10.1.2.3:50000", forwarded), "not believed from a remote peer")
	assert.Equal(t, netip.MustParseAddr("127.0.0.1"), call("127.0.0.1:50000", nil))
	assert.Equal(t, netip.MustParseAddr("127.0.0.1"),
		call("127.0.0.1:50000", metadata.Pairs(clientIPMetadata, "203.0.113.9", clientIPMetadata, "198.51.100.1")),
		"two values: one came from the client")
}

// A client cannot hand the grpc server its own address through a
// Grpc-Metadata-* header (the default matcher of grpc-gateway drops those):
// the gateway passes on only the resolved one.
func TestGrpcGatewayClientIPMetadata(t *testing.T) {
	_, ok := runtime.DefaultHeaderMatcher("Grpc-Metadata-X-Ruto-Client-Ip")
	assert.False(t, ok)

	req := httptest.NewRequest(http.MethodPost, "http://ruto.example.com/api/usr/login", nil)
	md := grpcGatewayMetadata(clientip.New(nil, nil))(context.Background(), req)
	assert.Equal(t, []string{"192.0.2.1"}, md.Get(clientIPMetadata))
}
