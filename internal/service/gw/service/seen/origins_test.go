package seen

import (
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeOrigin(t *testing.T) {
	ok := map[string]string{
		"https://App.Example.com":     "https://app.example.com",
		"http://localhost:5173":       "http://localhost:5173",
		" https://a.example.com/ ":    "https://a.example.com",
		"HTTPS://a.example.com:8443":  "https://a.example.com:8443",
		"http://[::1]:3000":           "http://[::1]:3000",
		"https://xn--d1acufc.example": "https://xn--d1acufc.example",
	}
	for raw, want := range ok {
		got, valid := NormalizeOrigin(raw)
		require.True(t, valid, raw)
		assert.Equal(t, want, got, raw)
	}

	for _, raw := range []string{
		"", "null", "*", "https://*.example.com", "ftp://x.example", "example.com",
		"https://a.example.com/api", "https://a.example.com?x=1", "https://a.example.com#f",
		"https://user@a.example.com", "https://" + string(make([]byte, maxOriginLen)),
	} {
		_, valid := NormalizeOrigin(raw)
		assert.False(t, valid, raw)
	}
}

// counted waits for the background goroutine to process the queue.
func counted(t *testing.T, s *Origins, want int64) map[string]*OriginItem {
	t.Helper()
	var byOrigin map[string]*OriginItem
	require.Eventually(t, func() bool {
		s.mu.Lock()
		defer s.mu.Unlock()
		var hits int64
		byOrigin = make(map[string]*OriginItem, len(s.items))
		for k, v := range s.items {
			byOrigin[k] = &OriginItem{Origin: v.Origin, Hits: v.Hits, LastSeenAt: v.LastSeenAt}
			hits += v.Hits
		}
		return hits == want
	}, 2*time.Second, time.Millisecond)
	return byOrigin
}

func TestOrigins_RecordDrainRestore(t *testing.T) {
	s := NewOrigins()
	s.Record("https://A.example.com")
	s.Record("https://a.example.com/")
	s.Record("null")
	s.Record("http://localhost:5173")

	byOrigin := counted(t, s, 3)
	require.Len(t, byOrigin, 2)
	require.Equal(t, int64(2), byOrigin["https://a.example.com"].Hits)
	require.Equal(t, int64(1), byOrigin["http://localhost:5173"].Hits)

	items := s.Drain()
	require.Len(t, items, 2)
	require.Empty(t, s.Drain())

	// counters survive a failed delivery and merge with new requests
	s.Restore(items)
	s.Record("http://localhost:5173")
	byOrigin = counted(t, s, 4)
	assert.Equal(t, int64(2), byOrigin["http://localhost:5173"].Hits)
	assert.Zero(t, s.Dropped())
}

// distinct waits until the map holds want origins.
func distinct(t *testing.T, s *Origins, want int) {
	t.Helper()
	require.Eventually(t, func() bool {
		s.mu.Lock()
		defer s.mu.Unlock()
		return len(s.items) == want
	}, 2*time.Second, time.Millisecond)
}

func TestOrigins_Cap(t *testing.T) {
	s := NewOrigins()
	s.Record("https://known.example")
	counted(t, s, 1)

	// which of the new origins survive a batch over the cap is up to map order,
	// so only their number is checked
	for i := 0; i < MaxOrigins+50; i++ {
		s.Record("https://h" + strconv.Itoa(i) + ".example")
	}
	distinct(t, s, MaxOrigins)

	// a known origin still counts once the cap is reached
	s.Record("https://known.example")
	require.Eventually(t, func() bool {
		s.mu.Lock()
		defer s.mu.Unlock()
		return s.items["https://known.example"].Hits == 2
	}, 2*time.Second, time.Millisecond)
	distinct(t, s, MaxOrigins)
}

func TestOrigins_LastSeen(t *testing.T) {
	s := NewOrigins()
	old := time.Now().Add(-time.Hour)
	s.Restore([]*OriginItem{{Origin: "https://a.example", Hits: 1, LastSeenAt: old}})
	s.Record("https://a.example")
	byOrigin := counted(t, s, 2)
	assert.True(t, byOrigin["https://a.example"].LastSeenAt.After(old))
}

// A full queue must drop instead of blocking the request.
func TestOrigins_FullQueueDrops(t *testing.T) {
	s := &Origins{queue: make(chan string, 2), items: make(map[string]*OriginItem)} // no goroutine
	done := make(chan struct{})
	go func() {
		for i := 0; i < 10; i++ {
			s.Record("https://a.example")
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Record blocked on a full queue")
	}
	assert.Equal(t, int64(8), s.Dropped())
}

// Records queued together are folded into one map update; hits of the same
// origin add up and the cap still holds for a batch of new origins.
func TestOrigins_BatchFold(t *testing.T) {
	s := &Origins{queue: make(chan string, originQueueSize), items: make(map[string]*OriginItem)} // goroutine started below
	for i := 0; i < 300; i++ {
		s.Record("https://a.example")
		s.Record("https://b" + strconv.Itoa(i%3) + ".example")
		s.Record("junk")
	}
	go s.run()

	byOrigin := counted(t, s, 600)
	require.Len(t, byOrigin, 4)
	assert.Equal(t, int64(300), byOrigin["https://a.example"].Hits)
	assert.Equal(t, int64(100), byOrigin["https://b0.example"].Hits)

	// a batch larger than the cap keeps only MaxOrigins distinct origins
	s2 := &Origins{queue: make(chan string, originQueueSize), items: make(map[string]*OriginItem)}
	for i := 0; i < MaxOrigins+100; i++ {
		s2.Record("https://h" + strconv.Itoa(i) + ".example")
	}
	go s2.run()
	byOrigin = counted(t, s2, MaxOrigins)
	require.Len(t, byOrigin, MaxOrigins)
}
