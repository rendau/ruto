package cloudflare

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const goodBody = `{"result":{"ipv4_cidrs":["203.0.113.0/24"],"ipv6_cidrs":["2001:db8::/32"],"etag":"v2"},"success":true}`

func newTestService(t *testing.T, handler http.HandlerFunc) (*Service, *atomic.Int32) {
	t.Helper()
	var calls atomic.Int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		handler(w, r)
	}))
	t.Cleanup(srv.Close)

	s := New(srv.URL, time.Hour)
	// the test server's certificate is trusted by its own client only
	s.httpClient = srv.Client()
	return s, &calls
}

func TestBuiltin(t *testing.T) {
	s := New("", 0)
	assert.True(t, s.Contains(netip.MustParseAddr("172.68.10.5")))
	assert.True(t, s.Contains(netip.MustParseAddr("::ffff:172.68.10.5")))
	assert.True(t, s.Contains(netip.MustParseAddr("2606:4700::1")))
	assert.False(t, s.Contains(netip.MustParseAddr("95.56.1.2")))
	assert.False(t, s.Contains(netip.Addr{}))
}

func TestRefreshReplacesList(t *testing.T) {
	s, _ := newTestService(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(goodBody))
	})

	require.NoError(t, s.refresh(t.Context()))
	assert.True(t, s.Contains(netip.MustParseAddr("203.0.113.7")))
	assert.True(t, s.Contains(netip.MustParseAddr("2001:db8::1")))
	assert.False(t, s.Contains(netip.MustParseAddr("172.68.10.5")))
	assert.Len(t, s.Networks(), 2)
}

func TestRefreshKeepsListOnBadAnswer(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
	}{
		{"server error", http.StatusInternalServerError, goodBody},
		{"not json", http.StatusOK, "<html>"},
		{"not successful", http.StatusOK, `{"result":{"ipv4_cidrs":["203.0.113.0/24"]},"success":false}`},
		{"no ipv4", http.StatusOK, `{"result":{"ipv4_cidrs":[],"ipv6_cidrs":["2001:db8::/32"]},"success":true}`},
		{"not a network", http.StatusOK, `{"result":{"ipv4_cidrs":["203.0.113.0/24","garbage"]},"success":true}`},
		{"whole internet v4", http.StatusOK, `{"result":{"ipv4_cidrs":["0.0.0.0/0"]},"success":true}`},
		{"too broad v4", http.StatusOK, `{"result":{"ipv4_cidrs":["10.0.0.0/7"]},"success":true}`},
		{"whole internet v6", http.StatusOK, `{"result":{"ipv4_cidrs":["203.0.113.0/24"],"ipv6_cidrs":["::/0"]},"success":true}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, _ := newTestService(t, func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			})

			require.Error(t, s.refresh(t.Context()))
			assert.True(t, s.Contains(netip.MustParseAddr("172.68.10.5")))
			assert.False(t, s.Contains(netip.MustParseAddr("203.0.113.7")))
		})
	}
}

func TestRefreshSkipsSameEtag(t *testing.T) {
	var body atomic.Pointer[string]
	body.Store(new(goodBody))
	s, _ := newTestService(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(*body.Load()))
	})
	require.NoError(t, s.refresh(t.Context()))
	before := s.networks.Load()

	require.NoError(t, s.refresh(t.Context()))
	assert.Same(t, before, s.networks.Load())

	body.Store(new(`{"result":{"ipv4_cidrs":["198.51.100.0/24"],"etag":"v3"},"success":true}`))
	require.NoError(t, s.refresh(t.Context()))
	assert.True(t, s.Contains(netip.MustParseAddr("198.51.100.1")))
	assert.False(t, s.Contains(netip.MustParseAddr("203.0.113.7")))
}

func TestRefreshVerifiesCertificate(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(goodBody))
	}))
	defer srv.Close()

	s := New(srv.URL, time.Hour)
	require.Error(t, s.refresh(t.Context()))
	assert.False(t, s.Contains(netip.MustParseAddr("203.0.113.7")))
}

func TestRunOff(t *testing.T) {
	s, calls := newTestService(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(goodBody))
	})
	s.interval = 0

	s.Run(t.Context()) // returns at once
	assert.Zero(t, calls.Load())
	assert.True(t, s.Contains(netip.MustParseAddr("172.68.10.5")))
}

func TestRunRefreshesAtStart(t *testing.T) {
	s, calls := newTestService(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(goodBody))
	})

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		s.Run(ctx)
		close(done)
	}()

	require.Eventually(t, func() bool { return s.Contains(netip.MustParseAddr("203.0.113.7")) }, 5*time.Second, 10*time.Millisecond)
	cancel()
	<-done
	assert.Equal(t, int32(1), calls.Load())
}
