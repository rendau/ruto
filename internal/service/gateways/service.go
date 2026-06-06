// Package gateways keeps an in-memory pool of currently connected gateways and
// lets core push notifications to them. Each connected gateway holds a
// long-lived gRPC stream to core; the stream handler registers a notify
// callback here, and core invokes those callbacks (e.g. after a snapshot
// deploy) to make the gateways re-check the snapshot version immediately
// instead of waiting for the next poll.
package gateways

import "sync"

// Service is a thread-safe registry of connected gateways' notify callbacks.
type Service struct {
	mu   sync.Mutex
	seq  int64
	subs map[int64]subscriber
}

type subscriber struct {
	gatewayID string
	notify    func()
}

func New() *Service {
	return &Service{
		subs: make(map[int64]subscriber),
	}
}

// Register adds a notify callback for a connected gateway and returns an opaque
// subscription id used to remove it on disconnect. The callback must be
// non-blocking (it is called while iterating the pool).
func (s *Service) Register(gatewayID string, notify func()) int64 {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.seq++
	id := s.seq
	s.subs[id] = subscriber{gatewayID: gatewayID, notify: notify}

	return id
}

// Unregister removes a previously registered subscription (on disconnect).
func (s *Service) Unregister(id int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.subs, id)
}

// NotifyAll invokes every registered callback. Callbacks are collected under
// the lock and invoked outside of it, so a callback can safely re-enter the
// pool and a slow callback can not block registrations.
func (s *Service) NotifyAll() {
	s.mu.Lock()
	notifiers := make([]func(), 0, len(s.subs))
	for _, sub := range s.subs {
		notifiers = append(notifiers, sub.notify)
	}
	s.mu.Unlock()

	for _, notify := range notifiers {
		notify()
	}
}

// Count returns the number of currently connected gateways.
func (s *Service) Count() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return len(s.subs)
}
