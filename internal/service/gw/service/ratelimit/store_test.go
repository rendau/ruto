package ratelimit

import (
	"net/http"
	"net/netip"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domAppModel "github.com/rendau/ruto/internal/domain/app/model"
	domEndpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
	rateLimitModel "github.com/rendau/ruto/internal/domain/ratelimit/model"
	"github.com/rendau/ruto/internal/service/gw/service/clientip"
)

type fakeClock struct {
	t time.Time
}

func (c *fakeClock) now() time.Time { return c.t }

func (c *fakeClock) add(d time.Duration) { c.t = c.t.Add(d) }

func newTestStore(maxKeys int) (*Store, *fakeClock) {
	clock := &fakeClock{t: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	return newStore(maxKeys, clock.now), clock
}

func takeN(s *Store, key uint64, limit int, period time.Duration, n int) int {
	passed := 0
	for range n {
		if ok, _, _ := s.take(key, limit, period, true); ok {
			passed++
		}
	}
	return passed
}

func TestStore_LimitWithinWindow(t *testing.T) {
	s, clock := newTestStore(100)

	assert.Equal(t, 3, takeN(s, 1, 3, 10*time.Second, 5))

	clock.add(2 * time.Second)
	ok, _, wait := s.take(1, 3, 10*time.Second, true)
	require.False(t, ok)
	// all three sit in the current window: they decay only from the next one
	assert.Equal(t, 8*time.Second, wait)
	assert.Equal(t, 8, RetryAfterSeconds(wait))

	// another key is counted on its own
	assert.Equal(t, 3, takeN(s, 2, 3, 10*time.Second, 3))
}

func TestStore_NoDoubleBurstAtWindowBoundary(t *testing.T) {
	s, clock := newTestStore(100)

	// the whole limit at the very end of a window
	clock.add(9900 * time.Millisecond)
	assert.Equal(t, 10, takeN(s, 1, 10, 10*time.Second, 10))

	// right after the boundary a fixed window would allow 10 more
	clock.add(200 * time.Millisecond)
	assert.Equal(t, 1, takeN(s, 1, 10, 10*time.Second, 10))

	// the previous window decays: 30% into the window 3 requests fit in all
	clock.add(2900 * time.Millisecond)
	assert.Equal(t, 2, takeN(s, 1, 10, 10*time.Second, 10))
}

func TestStore_RetryAfterFromPreviousWindow(t *testing.T) {
	s, clock := newTestStore(100)

	assert.Equal(t, 4, takeN(s, 1, 4, 10*time.Second, 4))
	clock.add(10 * time.Second)

	ok, _, wait := s.take(1, 4, 10*time.Second, true)
	require.False(t, ok)
	// 4*(10-x)/10 < 4 holds right after the window start
	assert.LessOrEqual(t, wait, time.Nanosecond)
	assert.Equal(t, 1, RetryAfterSeconds(wait))

	clock.add(2 * time.Second)
	ok, _, _ = s.take(1, 4, 10*time.Second, true)
	assert.True(t, ok)
	// 4*0.8 + 1 >= 4; one more fits once 4*(10-x)/10 + 1 < 4, at x > 2.5s
	ok, _, wait = s.take(1, 4, 10*time.Second, true)
	require.False(t, ok)
	assert.Equal(t, 500*time.Millisecond, wait)
}

func TestStore_RejectedRequestsAreNotCounted(t *testing.T) {
	s, clock := newTestStore(100)

	assert.Equal(t, 2, takeN(s, 1, 2, 10*time.Second, 100))
	clock.add(20 * time.Second)
	assert.Equal(t, 2, takeN(s, 1, 2, 10*time.Second, 2))
}

func TestStore_PeekDoesNotCount(t *testing.T) {
	s, _ := newTestStore(100)

	for range 5 {
		ok, _, _ := s.take(1, 1, time.Minute, false)
		require.True(t, ok)
	}
	assert.Equal(t, 0, s.Len())
	assert.Equal(t, 1, takeN(s, 1, 1, time.Minute, 2))
}

func TestStore_Undo(t *testing.T) {
	s, _ := newTestStore(100)

	ok, window, _ := s.take(1, 1, time.Minute, true)
	require.True(t, ok)
	s.undo(1, window)

	assert.Equal(t, 0, s.Len())
	assert.Equal(t, 1, takeN(s, 1, 1, time.Minute, 2))
}

func TestStore_Sweep(t *testing.T) {
	s, clock := newTestStore(100)

	takeN(s, 1, 5, 10*time.Second, 1)
	takeN(s, 2, 5, time.Minute, 1)
	require.Equal(t, 2, s.Len())

	// key 1 still matters while its window is the previous one
	clock.add(15 * time.Second)
	s.Sweep()
	assert.Equal(t, 2, s.Len())

	clock.add(10 * time.Second)
	s.Sweep()
	assert.Equal(t, 1, s.Len())

	clock.add(2 * time.Minute)
	s.Sweep()
	assert.Equal(t, 0, s.Len())
}

func TestStore_Shards(t *testing.T) {
	assert.Len(t, New(1_000_000).shards, 64)
	assert.Len(t, New(3000).shards, 2)
	assert.Len(t, New(3).shards, 1)
	assert.Equal(t, 15625, New(1_000_000).perShard)
}

func TestStore_KeyCapEvictsLeastUsed(t *testing.T) {
	s, _ := newTestStore(3)

	// key 1 is at its limit — a client being throttled
	assert.Equal(t, 2, takeN(s, 1, 2, time.Minute, 2))
	takeN(s, 2, 100, time.Minute, 1)
	takeN(s, 3, 100, time.Minute, 1)

	for key := uint64(10); key < 1000; key++ {
		ok, _, _ := s.take(key, 100, time.Minute, true)
		require.True(t, ok, "a new key passes at the cap")
		require.LessOrEqual(t, s.Len(), 3)
	}

	ok, _, _ := s.take(1, 2, time.Minute, true)
	assert.False(t, ok, "the throttled key keeps its counter")
}

func TestStore_KeyCapPrefersExpired(t *testing.T) {
	s, clock := newTestStore(2)

	assert.Equal(t, 1, takeN(s, 1, 1, time.Second, 1))
	clock.add(30 * time.Second)
	assert.Equal(t, 2, takeN(s, 2, 2, time.Minute, 2))

	takeN(s, 3, 100, time.Minute, 1)

	ok, _, _ := s.take(2, 2, time.Minute, true)
	assert.False(t, ok, "the expired key went, not the live one")
}

func TestRetryAfterSeconds(t *testing.T) {
	assert.Equal(t, 1, RetryAfterSeconds(0))
	assert.Equal(t, 1, RetryAfterSeconds(time.Millisecond))
	assert.Equal(t, 1, RetryAfterSeconds(time.Second))
	assert.Equal(t, 2, RetryAfterSeconds(time.Second+time.Nanosecond))
}

func TestLimitAddr(t *testing.T) {
	cases := map[string]string{
		"203.0.113.7":                 "203.0.113.7",
		"::ffff:203.0.113.7":          "203.0.113.7",
		"2001:db8:1:2:aaaa:bbbb:cc:d": "2001:db8:1:2::",
		"2001:db8:1:2::1%eth0":        "2001:db8:1:2::",
	}
	for in, want := range cases {
		t.Run(in, func(t *testing.T) {
			assert.Equal(t, want, limitAddr(mustAddr(t, in)).String())
		})
	}
}

func TestEndpoint_RejectedRequestIsNotCountedByOtherRules(t *testing.T) {
	s, _ := newTestStore(100)
	service := NewService(s, clientip.New(nil))

	gw := &rateLimitModel.Rule{Key: "ip", Requests: 3, Period: "1m", ParsedPeriod: time.Minute, Scope: rateLimitModel.ScopeRoot}
	login := &rateLimitModel.Rule{Key: "ip", Requests: 1, Period: "1m", ParsedPeriod: time.Minute, Scope: rateLimitModel.ScopeEndpoint("login")}
	app := &domAppModel.App{Id: "a", Name: "a"}

	loginEp := service.Endpoint(nil, app, &domEndpointModel.Endpoint{Id: "login", RateLimit: rateLimitModel.RateLimit{Rules: []*rateLimitModel.Rule{gw, login}}}, "POST /a/login")
	otherEp := service.Endpoint(nil, app, &domEndpointModel.Endpoint{Id: "other", RateLimit: rateLimitModel.RateLimit{Rules: []*rateLimitModel.Rule{gw}}}, "GET /a/other")
	require.NotNil(t, loginEp)

	req := Request{RemoteAddr: "203.0.113.7:1000", Header: http.Header{}}
	for i := range 5 {
		ok, _ := loginEp.Check(req)
		assert.Equal(t, i == 0, ok)
	}

	// only the one login that passed took the gateway-wide quota
	for i := range 3 {
		ok, _ := otherEp.Check(req)
		assert.Equal(t, i < 2, ok)
	}
}

func TestEndpoint_NoRules(t *testing.T) {
	s, _ := newTestStore(100)
	service := NewService(s, clientip.New(nil))

	assert.Nil(t, service.Endpoint(nil, &domAppModel.App{}, &domEndpointModel.Endpoint{}, ""))

	var none *Service
	assert.Nil(t, none.Endpoint(nil, &domAppModel.App{}, &domEndpointModel.Endpoint{}, ""))
}

func mustAddr(t *testing.T, s string) netip.Addr {
	t.Helper()
	addr, err := netip.ParseAddr(s)
	require.NoError(t, err)
	return addr
}

func BenchmarkEndpoint_Check(b *testing.B) {
	service := NewService(New(1_000_000), clientip.New(nil))
	ep := service.Endpoint(nil, &domAppModel.App{Id: "a"}, &domEndpointModel.Endpoint{Id: "e", RateLimit: rateLimitModel.RateLimit{Rules: []*rateLimitModel.Rule{
		{Key: "ip", Requests: 1_000_000_000, ParsedPeriod: time.Minute, Scope: "root"},
		{Key: "token", Requests: 1_000_000_000, ParsedPeriod: time.Minute, Scope: "endpoint:e"},
	}}}, "")
	req := Request{RemoteAddr: "203.0.113.7:1000", Header: http.Header{}, Authorization: "Bearer abc"}

	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			ep.Check(req)
		}
	})
}

func TestStore_Concurrent(t *testing.T) {
	s := New(1000)

	var passed atomic.Int64
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for range 1000 {
				if ok, _, _ := s.take(1, 500, time.Minute, true); ok {
					passed.Add(1)
				}
			}
		})
	}
	wg.Wait()

	assert.Equal(t, int64(500), passed.Load())
}
