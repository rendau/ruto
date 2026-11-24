// Package ratelimit counts the requests of clients against the rate-limit
// rules of endpoints.
//
// Counting is a sliding window counter: a counter keeps the number of requests
// of the current fixed window and of the previous one, and the previous one
// counts in proportion to how much of it still overlaps the sliding window
// that ends now. A key costs a fixed few dozen bytes whatever its limit, and
// unlike a plain fixed window a client cannot send a full limit at the end of
// one window and another one right after the boundary. Rejected requests are
// not counted.
//
// Store lives in gw.Service: handlers are rebuilt on every config change and
// the counters outlive them. The number of keys is capped (an attacker
// rotating addresses or tokens must not exhaust the memory): at the cap a new
// key evicts the least used of a few sampled keys of its shard, so the limits
// keep working for everyone and throttled clients, being near their limit,
// are the last to lose their counters.
package ratelimit

import (
	"context"
	"hash/maphash"
	"log/slog"
	"maps"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rendau/ruto/internal/service/gw/service/metrics"
)

const (
	maxShards       = 64
	minKeysPerShard = 1024
	evictSample     = 8
	evictWarnEvery  = time.Minute
)

type entry struct {
	start    int64 // the current window start, ns since Store.base
	periodMs uint32
	limit    uint32
	prev     uint32
	curr     uint32
}

type shard struct {
	mu    sync.Mutex
	items map[uint64]entry
	peak  int
	// keeps the hot mutexes of neighbouring shards off one cache line
	_ [40]byte
}

type Store struct {
	seed       maphash.Seed
	shards     []shard
	shardShift uint
	perShard   int
	maxKeys    int

	base time.Time
	now  func() time.Time

	evictWarn throttle
}

func New(maxKeys int) *Store {
	return newStore(maxKeys, time.Now)
}

func newStore(maxKeys int, now func() time.Time) *Store {
	maxKeys = max(maxKeys, 1)

	count, bits := 1, uint(0)
	for count < maxShards && maxKeys/(count*2) >= minKeysPerShard {
		count *= 2
		bits++
	}

	s := &Store{
		seed:       maphash.MakeSeed(),
		shards:     make([]shard, count),
		shardShift: 64 - bits,
		perShard:   maxKeys / count,
		maxKeys:    maxKeys,
		base:       now(),
		now:        now,
		evictWarn:  throttle{every: evictWarnEvery},
	}
	for i := range s.shards {
		s.shards[i].items = make(map[uint64]entry)
	}

	return s
}

// take checks the counter of key against limit per period and, with commit,
// counts the request when it passes. It returns the window the request was
// counted in (for undo) or, when rejected, how long until a request passes.
func (s *Store) take(key uint64, limit int, period time.Duration, commit bool) (bool, int64, time.Duration) {
	now := s.clock()
	p := int64(period)
	windowStart := now - now%p

	sh := s.shard(key)
	sh.mu.Lock()

	e, found := sh.items[key]
	if found {
		e.roll(windowStart, p)
	} else {
		e = entry{start: windowStart, periodMs: uint32(period.Milliseconds()), limit: uint32(limit)}
	}

	if e.estimate(now, p) >= float64(limit) {
		sh.mu.Unlock()
		return false, 0, e.retryAfter(now, p, limit)
	}
	if !commit {
		sh.mu.Unlock()
		return true, 0, 0
	}

	evicted := false
	if !found && len(sh.items) >= s.perShard {
		evicted = sh.evict(now)
	}
	e.curr++
	sh.items[key] = e
	sh.peak = max(sh.peak, len(sh.items))

	sh.mu.Unlock()

	if evicted {
		s.noteEviction()
	}

	return true, windowStart, 0
}

// undo takes back a request counted by take in window.
func (s *Store) undo(key uint64, window int64) {
	sh := s.shard(key)
	sh.mu.Lock()
	defer sh.mu.Unlock()

	e, ok := sh.items[key]
	if !ok || e.start != window || e.curr == 0 {
		return
	}
	e.curr--
	if e.curr == 0 && e.prev == 0 {
		delete(sh.items, key)
		return
	}
	sh.items[key] = e
}

// Len is the number of keys in the store.
func (s *Store) Len() int {
	total := 0
	for i := range s.shards {
		sh := &s.shards[i]
		sh.mu.Lock()
		total += len(sh.items)
		sh.mu.Unlock()
	}
	return total
}

// Sweep drops the counters whose both windows are over.
func (s *Store) Sweep() {
	now := s.clock()
	total := 0

	for i := range s.shards {
		sh := &s.shards[i]
		sh.mu.Lock()
		maps.DeleteFunc(sh.items, func(_ uint64, e entry) bool { return e.expired(now) })
		// A Go map never gives memory back: after a flood of keys the shard
		// is moved to a map of its live size.
		if sh.peak >= minKeysPerShard && len(sh.items) < sh.peak/4 {
			items := make(map[uint64]entry, len(sh.items))
			maps.Copy(items, sh.items)
			sh.items = items
			sh.peak = len(items)
		}
		total += len(sh.items)
		sh.mu.Unlock()
	}

	metrics.SetRateLimitKeys(total)
}

// Run sweeps every interval until ctx is done.
func (s *Store) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.Sweep()
		}
	}
}

func (s *Store) clock() int64 {
	return int64(s.now().Sub(s.base))
}

func (s *Store) shard(key uint64) *shard {
	return &s.shards[key>>s.shardShift]
}

func (s *Store) noteEviction() {
	metrics.IncRateLimitEvicted()
	if ok, skipped := s.evictWarn.allow(); ok {
		slog.Warn(
			"rate limit: key table is full, least used counters are evicted (raise RATE_LIMIT_MAX_KEYS if it is not an attack)",
			"max_keys", s.maxKeys,
			"evicted", skipped+1,
		)
	}
}

// evict drops the least used of a few keys: map iteration starts at a random
// place, so they are a random sample.
func (sh *shard) evict(now int64) bool {
	var victim uint64
	best := math.Inf(1)
	n := 0
	for key, e := range sh.items {
		if score := e.usage(now); score < best {
			victim, best = key, score
			if score == 0 {
				break
			}
		}
		if n++; n >= evictSample {
			break
		}
	}
	if math.IsInf(best, 1) {
		return false
	}
	delete(sh.items, victim)
	return true
}

func (e *entry) roll(windowStart, period int64) {
	switch {
	case windowStart <= e.start:
		return
	case windowStart-e.start == period:
		e.prev, e.curr = e.curr, 0
	default:
		e.prev, e.curr = 0, 0
	}
	e.start = windowStart
}

func (e *entry) estimate(now, period int64) float64 {
	elapsed := now - e.start
	return float64(e.prev)*float64(period-elapsed)/float64(period) + float64(e.curr)
}

// retryAfter is how long until estimate drops below limit. The previous
// window's share decays linearly over the current window; when the current
// window alone holds the limit, its count becomes the previous one of the
// next window and decays there.
func (e *entry) retryAfter(now, period int64, limit int) time.Duration {
	n := float64(limit)
	p := float64(period)

	var at float64
	if float64(e.curr) < n {
		// prev*(p-x)/p + curr < n
		at = float64(e.start) + p - (n-float64(e.curr))*p/float64(e.prev)
	} else {
		// curr*(p-x)/p < n in the next window
		at = float64(e.start) + p + p*(1-n/float64(e.curr))
	}

	wait := time.Duration(math.Ceil(at)) - time.Duration(now)
	return max(wait, 1)
}

func (e *entry) expired(now int64) bool {
	return now >= e.start+2*e.period()
}

// usage is how much of its limit the key has spent, 0 for an expired key.
func (e *entry) usage(now int64) float64 {
	if e.expired(now) {
		return 0
	}
	p := e.period()
	rolled := *e
	rolled.roll(now-now%p, p)
	return rolled.estimate(now, p) / float64(max(e.limit, 1))
}

func (e *entry) period() int64 {
	return int64(e.periodMs) * int64(time.Millisecond)
}

// throttle lets an event be logged at most once per every.
type throttle struct {
	every   time.Duration
	last    atomic.Int64
	skipped atomic.Int64
}

// allow reports whether to log now and how many events were not logged since
// the last time.
func (t *throttle) allow() (bool, int64) {
	now := time.Now().UnixNano()
	last := t.last.Load()
	if (last != 0 && now-last < int64(t.every)) || !t.last.CompareAndSwap(last, now) {
		t.skipped.Add(1)
		return false, 0
	}
	return true, t.skipped.Swap(0)
}
