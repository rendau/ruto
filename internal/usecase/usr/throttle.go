package usr

import (
	"net/netip"
	"strings"
	"sync"
	"time"
)

// Sign-in throttle limits. An address gets throttlePairMax wrong passwords for
// one username and throttleAddrMax for all usernames within throttleWindow;
// after that every sign-in from it (for that username, or at all) is refused
// for throttleLockout, the right password included.
const (
	throttleWindow  = 15 * time.Minute
	throttleLockout = 15 * time.Minute
	throttlePairMax = 5
	throttleAddrMax = 20
)

// loginThrottle keeps password guessing slow. Failures are counted per
// address, not per username alone: otherwise anyone could lock an
// administrator out by typing wrong passwords for their username. The state is
// in memory, per core instance.
type loginThrottle struct {
	mu        sync.Mutex
	now       func() time.Time
	entries   map[string]*throttleEntry
	lastSweep time.Time
}

type throttleEntry struct {
	fails        int
	since        time.Time
	blockedUntil time.Time
}

func newLoginThrottle(now func() time.Time) *loginThrottle {
	return &loginThrottle{
		now:     now,
		entries: map[string]*throttleEntry{},
	}
}

func throttleKeys(username string, ip netip.Addr) (string, string) {
	addr := "addr|" + ip.String()
	return addr + "|" + strings.ToLower(username), addr
}

// Blocked reports whether a sign-in for username from ip is refused now.
func (t *loginThrottle) Blocked(username string, ip netip.Addr) bool {
	pairKey, addrKey := throttleKeys(username, ip)

	t.mu.Lock()
	defer t.mu.Unlock()

	now := t.now()
	t.sweep(now)

	for _, key := range []string{pairKey, addrKey} {
		if e, ok := t.entries[key]; ok && now.Before(e.blockedUntil) {
			return true
		}
	}
	return false
}

// Failed counts a wrong password for username from ip.
func (t *loginThrottle) Failed(username string, ip netip.Addr) {
	pairKey, addrKey := throttleKeys(username, ip)

	t.mu.Lock()
	defer t.mu.Unlock()

	now := t.now()
	t.fail(pairKey, throttlePairMax, now)
	t.fail(addrKey, throttleAddrMax, now)
}

// Succeeded forgets the failures for username from ip. The address count is
// kept: one account of one's own must not reset guessing at the others.
func (t *loginThrottle) Succeeded(username string, ip netip.Addr) {
	pairKey, _ := throttleKeys(username, ip)

	t.mu.Lock()
	defer t.mu.Unlock()

	delete(t.entries, pairKey)
}

func (t *loginThrottle) fail(key string, maxFails int, now time.Time) {
	e, ok := t.entries[key]
	if !ok || now.Sub(e.since) > throttleWindow {
		e = &throttleEntry{since: now}
		t.entries[key] = e
	}
	e.fails++
	if e.fails >= maxFails {
		e.blockedUntil = now.Add(throttleLockout)
	}
}

// sweep drops entries that neither block nor count any more, at most once a
// window, so addresses that tried once do not pile up.
func (t *loginThrottle) sweep(now time.Time) {
	if now.Sub(t.lastSweep) < throttleWindow {
		return
	}
	t.lastSweep = now
	for key, e := range t.entries {
		if now.Sub(e.since) > throttleWindow && !now.Before(e.blockedUntil) {
			delete(t.entries, key)
		}
	}
}
