package oidc

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	authModel "github.com/rendau/ruto/internal/domain/auth/model"
	snapshotModel "github.com/rendau/ruto/internal/domain/snapshot/model"
)

// sharedProvider counts token calls; "rt-dead" is refused, anything else is
// rotated after delay.
func sharedProvider(t *testing.T, delay time.Duration) (tokenEndpoint string, calls *atomic.Int32) {
	calls = new(atomic.Int32)
	var issuer string
	server := fakeProvider(t, func() string { return issuer }, func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		time.Sleep(delay)
		_ = r.ParseForm()
		if r.PostForm.Get("refresh_token") == "rt-dead" {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid_grant"})
			return
		}
		_ = json.NewEncoder(w).Encode(TokenResponse{AccessToken: "at-" + string(rune('0'+n)), RefreshToken: "rt-new"})
	})
	issuer = server.URL + "/realms/corp"
	return issuer + "/protocol/openid-connect/token", calls
}

// pods are gateway pods: separate services, so nothing is shared in memory.
func pods(t *testing.T, redisAddr string, n int) []*Service {
	result := make([]*Service, n)
	for i := range result {
		result[i] = New()
		result[i].SetRedis(&snapshotModel.GatewaySettings{RedisAddr: redisAddr})
		t.Cleanup(func() { result[i].SetRedis(&snapshotModel.GatewaySettings{}) })
	}
	return result
}

var sharedClient = &authModel.JwtClient{Id: "ruto", Secret: "s"}

func TestSharedRefreshAcrossPods(t *testing.T) {
	redisServer := miniredis.RunT(t)
	tokenEndpoint, calls := sharedProvider(t, 100*time.Millisecond)
	gateways := pods(t, redisServer.Addr(), 3)

	// three pods, a few requests each, all with the same refresh token at once
	const perPod = 4
	var wg sync.WaitGroup
	results := make([]*TokenResponse, len(gateways)*perPod)
	errs := make([]error, len(results))
	for i := range results {
		svc := gateways[i%len(gateways)]
		wg.Go(func() { results[i], errs[i] = svc.Refresh(context.Background(), tokenEndpoint, sharedClient, "rt-1") })
	}
	wg.Wait()

	assert.Equal(t, int32(1), calls.Load(), "one call to the provider from all pods")
	for i := range results {
		require.NoError(t, errs[i])
		assert.Equal(t, results[0].AccessToken, results[i].AccessToken)
	}

	// a pod that has not seen the refresh gets the answer from redis
	late := pods(t, redisServer.Addr(), 1)[0]
	rep, err := late.Refresh(context.Background(), tokenEndpoint, sharedClient, "rt-1")
	require.NoError(t, err)
	assert.Equal(t, results[0].AccessToken, rep.AccessToken)
	assert.Equal(t, int32(1), calls.Load())

	// the answer lives RefreshGrace in redis, the lock is gone
	keys := redisServer.Keys()
	require.Len(t, keys, 1)
	assert.Equal(t, RefreshGrace, redisServer.TTL(keys[0]))

	// after that the token goes to the provider again
	redisServer.FastForward(RefreshGrace)
	_, err = pods(t, redisServer.Addr(), 1)[0].Refresh(context.Background(), tokenEndpoint, sharedClient, "rt-1")
	require.NoError(t, err)
	assert.Equal(t, int32(2), calls.Load())
}

func TestSharedRefreshRefusalIsNotKept(t *testing.T) {
	redisServer := miniredis.RunT(t)
	tokenEndpoint, calls := sharedProvider(t, 0)
	gateways := pods(t, redisServer.Addr(), 2)

	for _, svc := range gateways {
		_, err := svc.Refresh(context.Background(), tokenEndpoint, sharedClient, "rt-dead")
		var refusal *Error
		require.ErrorAs(t, err, &refusal)
		assert.Equal(t, "invalid_grant", refusal.Code)
	}
	assert.Equal(t, int32(2), calls.Load(), "each pod hears the refusal from the provider")
	assert.Empty(t, redisServer.Keys(), "no answer and no lock left behind")
}

func TestSharedRefreshWithoutRedis(t *testing.T) {
	redisServer := miniredis.RunT(t)
	tokenEndpoint, calls := sharedProvider(t, 0)
	svc := pods(t, redisServer.Addr(), 1)[0]
	redisServer.Close()

	rep, err := svc.Refresh(context.Background(), tokenEndpoint, sharedClient, "rt-1")
	require.NoError(t, err, "a pod refreshes on its own when redis is down")
	assert.Equal(t, "rt-new", rep.RefreshToken)
	assert.Equal(t, int32(1), calls.Load())
}

func TestSharedRefreshLockOfAGoneHolder(t *testing.T) {
	redisServer := miniredis.RunT(t)
	tokenEndpoint, calls := sharedProvider(t, 0)
	svc := pods(t, redisServer.Addr(), 1)[0]

	// a pod took the lock and died: the lock expires, the next refresh
	// takes it over
	key := tokenEndpoint + "\x00" + sharedClient.Id + "\x00" + "rt-1"
	redisServer.Set(sharedAnswerKey(key)+":lock", "1")
	redisServer.SetTTL(sharedAnswerKey(key)+":lock", 50*time.Millisecond)
	go func() {
		time.Sleep(100 * time.Millisecond)
		redisServer.FastForward(time.Second)
	}()

	rep, err := svc.Refresh(context.Background(), tokenEndpoint, sharedClient, "rt-1")
	require.NoError(t, err)
	assert.Equal(t, "rt-new", rep.RefreshToken)
	assert.Equal(t, int32(1), calls.Load())
}
