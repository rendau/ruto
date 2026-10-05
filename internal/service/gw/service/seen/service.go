// Package seen counts which concrete paths wildcard endpoints actually serve,
// so the admin can later replace a wildcard with explicit endpoints. Paths are
// collapsed into templates (ids → {id}) and the number of distinct templates
// per endpoint is capped: the collector must stay small even under a scan
// that hits thousands of random URLs.
package seen

import (
	"strings"
	"sync"
	"time"
)

const (
	// MaxPathsPerEndpoint caps distinct templates kept per endpoint between two
	// drains; everything above goes to OtherPath.
	MaxPathsPerEndpoint = 1000

	// OtherPath aggregates requests that did not fit into the cap.
	OtherPath = "(other)"

	IdPlaceholder = "{id}"

	maxSegmentLen = 30
	maxSegments   = 12
	maxSampleLen  = 300
)

type Item struct {
	AppId        string
	EndpointId   string
	Method       string
	Path         string
	Hits         int64
	HitsNotFound int64
	LastSeenAt   time.Time
	Sample       string
}

type key struct {
	endpointId string
	method     string
	path       string
}

type Service struct {
	mu       sync.Mutex
	items    map[key]*Item
	perEpCnt map[string]int
}

var ins = New()

// Ins is the process-wide collector: it outlives handler rebuilds, which
// happen on every snapshot change.
func Ins() *Service {
	return ins
}

func New() *Service {
	return &Service{
		items:    make(map[key]*Item),
		perEpCnt: make(map[string]int),
	}
}

// Record counts one request. path is relative to the app prefix.
func (s *Service) Record(appId, endpointId, method, path string, notFound bool) {
	s.add(&Item{
		AppId:        appId,
		EndpointId:   endpointId,
		Method:       method,
		Path:         NormalizePath(path),
		Hits:         1,
		HitsNotFound: boolToInt(notFound),
		LastSeenAt:   time.Now(),
		Sample:       truncate(path, maxSampleLen),
	})
}

// Drain returns everything collected so far and starts from scratch.
func (s *Service) Drain() []*Item {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make([]*Item, 0, len(s.items))
	for _, item := range s.items {
		result = append(result, item)
	}
	s.items = make(map[key]*Item)
	s.perEpCnt = make(map[string]int)

	return result
}

// Restore puts drained items back, e.g. when they could not be delivered.
func (s *Service) Restore(items []*Item) {
	for _, item := range items {
		s.add(item)
	}
}

func (s *Service) add(v *Item) {
	s.mu.Lock()
	defer s.mu.Unlock()

	k := key{endpointId: v.EndpointId, method: v.Method, path: v.Path}
	item, ok := s.items[k]
	if !ok && s.perEpCnt[v.EndpointId] >= MaxPathsPerEndpoint {
		k.path = OtherPath
		item, ok = s.items[k]
	}
	if !ok {
		item = &Item{
			AppId:      v.AppId,
			EndpointId: v.EndpointId,
			Method:     v.Method,
			Path:       k.path,
		}
		s.items[k] = item
		if k.path != OtherPath {
			s.perEpCnt[v.EndpointId]++
		}
	}

	item.Hits += v.Hits
	item.HitsNotFound += v.HitsNotFound
	if v.LastSeenAt.After(item.LastSeenAt) {
		item.LastSeenAt = v.LastSeenAt
		item.Sample = v.Sample
	}
}

// NormalizePath turns a concrete path into a template: segments that look like
// identifiers (numbers, uuids, hashes, long slugs) become {id}.
func NormalizePath(path string) string {
	path = strings.Trim(path, "/")
	if path == "" {
		return ""
	}

	segments := strings.Split(path, "/")
	if len(segments) > maxSegments {
		segments = append(segments[:maxSegments], "...")
	}
	for i, segment := range segments {
		if isIdSegment(segment) {
			segments[i] = IdPlaceholder
		}
	}

	return strings.Join(segments, "/")
}

func isIdSegment(v string) bool {
	if v == "" {
		return false
	}
	if len(v) > maxSegmentLen {
		return true
	}

	digits, hexes := 0, 0
	for _, c := range v {
		switch {
		case c >= '0' && c <= '9':
			digits++
			hexes++
		case (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F') || c == '-':
			hexes++
		}
	}

	if digits == len(v) {
		return true
	}
	// uuids and hashes: hex-only, long enough, and not a plain word like "feed"
	return hexes == len(v) && len(v) >= 16 && digits > 0
}

func truncate(v string, limit int) string {
	if len(v) <= limit {
		return v
	}
	return v[:limit]
}

func boolToInt(v bool) int64 {
	if v {
		return 1
	}
	return 0
}
