// Package cache keeps backend responses in the gateway memory for endpoints
// with server cache enabled. It outlives handler rebuilds, which happen on
// every snapshot change. Expired entries are dropped by a background sweep, so
// the memory holds only live responses; maxSize is just the ceiling — when it
// is reached new responses are not cached until the sweep frees room.
package cache

import (
	"context"
	"net/http"
	"sync"
	"time"
)

type Entry struct {
	Status   int
	Header   http.Header
	Body     []byte
	StoredAt time.Time

	expiresAt time.Time
	size      int64
}

type Service struct {
	mu      sync.RWMutex
	items   map[string]*Entry
	size    int64
	maxSize int64
}

func New(maxSize int64) *Service {
	return &Service{
		items:   make(map[string]*Entry),
		maxSize: maxSize,
	}
}

// Get returns a live entry or nil.
func (s *Service) Get(key string) *Entry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry := s.items[key]
	if entry == nil || !time.Now().Before(entry.expiresAt) {
		return nil
	}
	return entry
}

// Set stores the entry for ttl. It returns false when the cache has no room.
func (s *Service) Set(key string, entry *Entry, ttl time.Duration) bool {
	entry.StoredAt = time.Now()
	entry.expiresAt = entry.StoredAt.Add(ttl)
	entry.size = entrySize(key, entry)

	s.mu.Lock()
	defer s.mu.Unlock()

	var oldSize int64
	if old := s.items[key]; old != nil {
		oldSize = old.size
	}
	if s.size-oldSize+entry.size > s.maxSize {
		return false
	}

	s.items[key] = entry
	s.size += entry.size - oldSize

	return true
}

// Size is the number of bytes the cached entries take.
func (s *Service) Size() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.size
}

// Sweep drops expired entries.
func (s *Service) Sweep() {
	now := time.Now()

	s.mu.Lock()
	defer s.mu.Unlock()

	for key, entry := range s.items {
		if !now.Before(entry.expiresAt) {
			delete(s.items, key)
			s.size -= entry.size
		}
	}
}

// Run sweeps every interval until ctx is done.
func (s *Service) Run(ctx context.Context, interval time.Duration) {
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

func entrySize(key string, entry *Entry) int64 {
	size := len(key) + len(entry.Body)
	for k, vs := range entry.Header {
		size += len(k)
		for _, v := range vs {
			size += len(v)
		}
	}
	return int64(size)
}
