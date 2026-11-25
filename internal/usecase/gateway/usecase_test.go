package gateway

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	gatewaysServiceP "github.com/rendau/ruto/internal/service/gateways"
)

func TestSubscribeNotificationTypes(t *testing.T) {
	pool := gatewaysServiceP.New()
	uc := New(nil, nil, pool, nil, nil, nil)

	var mu sync.Mutex
	var sent []string
	count := func(kind string) int {
		mu.Lock()
		defer mu.Unlock()
		n := 0
		for _, v := range sent {
			if v == kind {
				n++
			}
		}
		return n
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- uc.Subscribe(ctx, "gw-1", func(kind string) error {
			mu.Lock()
			sent = append(sent, kind)
			mu.Unlock()
			return nil
		})
	}()

	// both are sent right after (re)connect
	require.Eventually(t, func() bool {
		return count(NotificationCheckVersion) == 1 && count(NotificationCheckBlocklist) == 1
	}, time.Second, 5*time.Millisecond)

	uc.NotifyBlocklist()
	require.Eventually(t, func() bool { return count(NotificationCheckBlocklist) == 2 }, time.Second, 5*time.Millisecond)
	assert.Equal(t, 1, count(NotificationCheckVersion), "a block list change does not make gateways re-check the snapshot")

	pool.NotifyAll()
	require.Eventually(t, func() bool { return count(NotificationCheckVersion) == 2 }, time.Second, 5*time.Millisecond)
	assert.Equal(t, 2, count(NotificationCheckBlocklist))

	cancel()
	require.NoError(t, <-done)

	uc.blocklistMu.Lock()
	assert.Empty(t, uc.blocklistSubs, "a closed stream unsubscribes")
	uc.blocklistMu.Unlock()
	assert.Zero(t, pool.Count())
}
