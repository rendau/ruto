package core

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	sessionModel "github.com/rendau/ruto/internal/domain/session/model"
	sessionService "github.com/rendau/ruto/internal/domain/session/service"
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

// An api key works on the main API only for BlockedClient List/Create/Remove:
// everywhere else it must not even be looked up.
func TestSessionApiKeyAllowlist(t *testing.T) {
	sessionSvc := sessionService.New("test-secret")
	keyAuth := &testApiKeyAuth{}
	interceptor := GrpcInterceptorSession(sessionSvc, keyAuth)

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
	} {
		assert.False(t, call(method, "Bearer "+testApiKey).IsAuthorized(), method)
	}
	assert.Zero(t, keyAuth.calls, "the key is not even checked outside the allowlist")

	// the admin login keeps working everywhere
	token, err := sessionSvc.CreateToken(7, true, true, nil)
	require.NoError(t, err)
	for _, method := range []string{ruto_v1.Root_Get_FullMethodName, ruto_v1.BlockedClient_Create_FullMethodName} {
		got := call(method, "Bearer "+token)
		assert.True(t, got.IsHuman(), method)
		assert.Equal(t, int64(7), got.Id)
	}
}
