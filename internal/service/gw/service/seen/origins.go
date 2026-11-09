package seen

import (
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	// MaxOrigins caps distinct origins kept between two drains: the header is
	// set by the client, so a scan must not grow the collector without bound.
	// Everything above is dropped, not aggregated — an unnamed origin is of no
	// use for an allow list.
	MaxOrigins = 500

	// originQueueSize is how many raw headers may wait for the counting
	// goroutine. A full queue drops the record instead of delaying the
	// request: this is telemetry, the request path must never wait for it.
	originQueueSize = 4096

	// originBatchSize bounds one pass of the counting goroutine, so a flood
	// cannot keep the lock away from Drain for long.
	originBatchSize = 1024

	maxOriginLen = 300
)

// OriginItem is one browser origin (scheme://host[:port]) seen in requests.
type OriginItem struct {
	Origin     string
	Hits       int64
	LastSeenAt time.Time
}

// Origins collects the Origin header of incoming requests, so the admin can
// pick the allow_origins of CORS from what actually calls the gateway instead
// of guessing.
//
// The request path only hands the raw header to a buffered channel; parsing,
// lower-casing and the map update run in one background goroutine, so a
// request never pays for them and never waits for another request's record.
type Origins struct {
	queue   chan string
	dropped atomic.Int64

	mu    sync.Mutex
	items map[string]*OriginItem
}

var originsIns = NewOrigins()

// OriginsIns is the process-wide collector: it outlives handler rebuilds.
func OriginsIns() *Origins {
	return originsIns
}

// NewOrigins starts the counting goroutine; it lives as long as the process,
// like the collector itself.
func NewOrigins() *Origins {
	s := &Origins{
		queue: make(chan string, originQueueSize),
		items: make(map[string]*OriginItem),
	}
	go s.run()
	return s
}

// Record queues one request's Origin header. It never blocks: when the queue
// is full the record is dropped and counted in Dropped.
func (s *Origins) Record(origin string) {
	select {
	case s.queue <- origin:
	default:
		s.dropped.Add(1)
	}
}

// Dropped is how many records were lost to a full queue since start.
func (s *Origins) Dropped() int64 {
	return s.dropped.Load()
}

// run blocks on the first record, then takes everything else already queued
// (up to originBatchSize), parses and folds the batch without the lock and
// merges it into the map under one Lock. Under load the lock is thus taken
// once per batch, not once per request.
func (s *Origins) run() {
	batch := make(map[string]*OriginItem, 64)
	for raw := range s.queue {
		now := time.Now()
		foldOrigin(batch, raw, now)
	drain:
		for len(batch) < originBatchSize {
			select {
			case raw, ok := <-s.queue:
				if !ok {
					break drain
				}
				foldOrigin(batch, raw, now)
			default:
				break drain
			}
		}
		s.addBatch(batch)
		clear(batch)
	}
}

func foldOrigin(batch map[string]*OriginItem, raw string, now time.Time) {
	origin, ok := NormalizeOrigin(raw)
	if !ok {
		return
	}
	if item, ok := batch[origin]; ok {
		item.Hits++
		item.LastSeenAt = now
		return
	}
	batch[origin] = &OriginItem{Origin: origin, Hits: 1, LastSeenAt: now}
}

// Drain returns everything counted so far and starts from scratch. Records
// still waiting in the queue are not included; they land in the next drain.
func (s *Origins) Drain() []*OriginItem {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make([]*OriginItem, 0, len(s.items))
	for _, item := range s.items {
		result = append(result, item)
	}
	s.items = make(map[string]*OriginItem)

	return result
}

// Restore puts drained items back, e.g. when they could not be delivered.
func (s *Origins) Restore(items []*OriginItem) {
	batch := make(map[string]*OriginItem, len(items))
	for _, item := range items {
		batch[item.Origin] = item
	}
	s.addBatch(batch)
}

// addBatch merges distinct origins into the map under a single lock.
func (s *Origins) addBatch(batch map[string]*OriginItem) {
	if len(batch) == 0 {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for origin, v := range batch {
		item, ok := s.items[origin]
		if !ok {
			if len(s.items) >= MaxOrigins {
				continue
			}
			item = &OriginItem{Origin: origin}
			s.items[origin] = item
		}

		item.Hits += v.Hits
		if v.LastSeenAt.After(item.LastSeenAt) {
			item.LastSeenAt = v.LastSeenAt
		}
	}
}

// NormalizeOrigin brings an Origin header value to the form allow_origins
// uses: lower-case "scheme://host[:port]". The second result is false for
// anything that is not an http(s) origin, including the "null" a browser
// sends for opaque origins.
func NormalizeOrigin(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" || len(raw) > maxOriginLen {
		return "", false
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", false
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", false
	}
	if parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" ||
		(parsed.Path != "" && parsed.Path != "/") || strings.Contains(parsed.Host, "*") {
		return "", false
	}
	return strings.ToLower(parsed.Scheme + "://" + parsed.Host), true
}
