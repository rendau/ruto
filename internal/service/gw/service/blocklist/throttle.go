package blocklist

import (
	"sync/atomic"
	"time"
)

// throttle lets a warning through at most once per every, so that a flood of
// blocked requests does not flood the log.
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
