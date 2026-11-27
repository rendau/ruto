package clientstat

import (
	"net/netip"
	"strconv"
	"sync"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func recordIP(c *Collector, ip string, d counter) {
	c.record(clientKey(netip.MustParseAddr(ip)), d)
}

func byIP(items []*Item) map[string]*Item {
	return lo.KeyBy(items, func(v *Item) string { return v.Trusted + "|" + v.Ip })
}

func totals(items []*Item) (int64, int64, int64) {
	return lo.SumBy(items, func(v *Item) int64 { return v.Requests }),
		lo.SumBy(items, func(v *Item) int64 { return v.Blocked }),
		lo.SumBy(items, func(v *Item) int64 { return v.Limited })
}

func TestCollector_Keys(t *testing.T) {
	c := NewCollector(100)
	recordIP(c, "203.0.113.7", counter{requests: 2})
	recordIP(c, "::ffff:203.0.113.7", counter{requests: 1})
	recordIP(c, "2001:db8:1:2::1", counter{requests: 1})
	recordIP(c, "2001:db8:1:2:ffff::9", counter{requests: 1, blocked: 1})
	recordIP(c, "fe80::1%eth0", counter{requests: 1})
	c.record(clientKey(netip.Addr{}), counter{requests: 4})
	c.record(key{prefix: netip.MustParsePrefix("10.0.0.0/8"), trusted: "lan"}, counter{requests: 5})
	c.record(key{prefix: netip.MustParsePrefix("10.1.2.3/32"), trusted: "ci"}, counter{requests: 6})

	items := c.Drain()
	got := byIP(items)
	require.Len(t, got, 6)
	assert.Equal(t, int64(3), got["|203.0.113.7"].Requests, "a mapped address is the plain one")
	assert.Equal(t, &Item{Ip: "2001:db8:1:2::/64", Requests: 2, Blocked: 1}, got["|2001:db8:1:2::/64"])
	assert.Equal(t, int64(1), got["|fe80::/64"].Requests, "the zone is dropped")
	assert.Equal(t, int64(4), got["|"].Requests)
	assert.Equal(t, int64(5), got["lan|10.0.0.0/8"].Requests)
	assert.Equal(t, int64(6), got["ci|10.1.2.3/32"].Requests)

	assert.Equal(t, "", items[len(items)-1].Ip, "the ip \"\" row goes last")
	assert.Empty(t, c.Drain(), "a drain starts from scratch")
}

func TestCollector_CapOverflowToUnknownRow(t *testing.T) {
	c := NewCollector(3)
	for i := range 5 {
		recordIP(c, "203.0.113."+strconv.Itoa(i+1), counter{requests: 1, limited: 1, lastRule: "r" + strconv.Itoa(i), ruleAt: int64(i)})
	}
	// known clients keep counting, trusted ones are never capped
	recordIP(c, "203.0.113.1", counter{requests: 1})
	c.record(key{prefix: netip.MustParsePrefix("10.0.0.0/8"), trusted: "lan"}, counter{requests: 1})

	items := c.Drain()
	got := byIP(items)
	require.Len(t, got, 5)
	assert.Equal(t, int64(2), got["|203.0.113.1"].Requests)
	assert.Equal(t, &Item{Requests: 2, Limited: 2, LastRule: "r4"}, got["|"], "the overflow, with its latest rule")
	assert.Contains(t, got, "lan|10.0.0.0/8")

	requests, _, limited := totals(items)
	assert.Equal(t, int64(7), requests)
	assert.Equal(t, int64(5), limited)

	// the drain frees the cap
	for i := range 3 {
		recordIP(c, "198.51.100."+strconv.Itoa(i+1), counter{requests: 1})
	}
	assert.NotContains(t, byIP(c.Drain()), "|")
}

func TestCollector_DrainBounded(t *testing.T) {
	c := NewCollector(MaxClients)
	const clients = 2000
	for i := range clients {
		ip := netip.AddrFrom4([4]byte{198, 18, byte(i >> 8), byte(i)}).String()
		// busy clients by requests: 1000..1999; refused ones: the quiet 0..399
		d := counter{requests: int64(i) + 1}
		if i < 400 {
			d.blocked = int64(400 - i)
		}
		recordIP(c, ip, d)
	}
	c.record(key{prefix: netip.MustParsePrefix("10.0.0.0/8"), trusted: "lan"}, counter{requests: 1})

	items := c.Drain()
	assert.Len(t, items, 1+2*ReportTop+1, "trusted, the top by requests, the top by refusals, the rest")

	got := byIP(items)
	assert.Contains(t, got, "lan|10.0.0.0/8")
	assert.Contains(t, got, "|198.18.7.207", "the busiest client (1999)")
	assert.Contains(t, got, "|198.18.6.164", "the last of the top by requests (1700)")
	assert.NotContains(t, got, "|198.18.6.163")
	assert.Contains(t, got, "|198.18.0.0", "the most refused client")
	assert.Contains(t, got, "|198.18.1.43", "the last of the top by refusals (299)")
	assert.NotContains(t, got, "|198.18.1.44")

	requests, blocked, _ := totals(items)
	assert.Equal(t, int64(clients*(clients+1)/2+1), requests, "totals stay exact")
	assert.Equal(t, int64(400*401/2), blocked)
}

func TestCollector_Restore(t *testing.T) {
	c := NewCollector(100)
	recordIP(c, "203.0.113.7", counter{requests: 3, preflights: 2, limited: 1, lastRule: "old", ruleAt: 1})
	recordIP(c, "2001:db8::1", counter{requests: 1})
	c.record(key{}, counter{requests: 2, lastRule: "x", ruleAt: 1})
	c.record(key{prefix: netip.MustParsePrefix("10.1.2.3/32"), trusted: "ci"}, counter{requests: 4})
	failed := c.Drain()

	recordIP(c, "203.0.113.7", counter{requests: 1, limited: 1, lastRule: "new", ruleAt: 2})
	c.Restore(failed)

	got := byIP(c.Drain())
	assert.Equal(t, &Item{Ip: "203.0.113.7", Requests: 4, Preflights: 2, Limited: 2, LastRule: "new"}, got["|203.0.113.7"])
	assert.Equal(t, int64(1), got["|2001:db8::/64"].Requests)
	assert.Equal(t, &Item{Requests: 2, LastRule: "x"}, got["|"])
	assert.Equal(t, &Item{Ip: "10.1.2.3/32", Trusted: "ci", Requests: 4}, got["ci|10.1.2.3/32"])

	c.Restore([]*Item{{Ip: "not an ip", Requests: 1}})
	assert.Equal(t, []*Item{{Requests: 1}}, c.Drain(), "an unreadable row lands in the ip \"\" row")
}

func TestCollector_ConcurrentTotalsExact(t *testing.T) {
	c := NewCollector(500)
	const workers, perWorker = 8, 5000

	var drained []*Item
	var mu sync.Mutex
	stop := make(chan struct{})
	var drainer sync.WaitGroup
	drainer.Go(func() {
		for {
			select {
			case <-stop:
				return
			default:
			}
			items := c.Drain()
			mu.Lock()
			drained = append(drained, items...)
			mu.Unlock()
		}
	})

	var wg sync.WaitGroup
	for w := range workers {
		wg.Go(func() {
			for i := range perWorker {
				ip := netip.AddrFrom4([4]byte{198, 18, byte(w), byte(i % 1000 / 4)})
				c.record(clientKey(ip), counter{requests: 1, blocked: int64(i % 2)})
			}
		})
	}
	wg.Wait()
	close(stop)
	drainer.Wait()
	drained = append(drained, c.Drain()...)

	requests, blocked, _ := totals(drained)
	assert.Equal(t, int64(workers*perWorker), requests)
	assert.Equal(t, int64(workers*perWorker/2), blocked)
	assert.Zero(t, c.size.Load(), "the cap count returns to zero")
}

func BenchmarkRecord(b *testing.B) {
	b.Run("few clients", func(b *testing.B) {
		c := NewCollector(MaxClients)
		keys := lo.Times(16, func(i int) key { return clientKey(netip.AddrFrom4([4]byte{203, 0, 113, byte(i)})) })
		b.ReportAllocs()
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				c.record(keys[i&15], counter{requests: 1})
				i++
			}
		})
	})

	// a flood from many addresses: the cap fills, the rest goes to the ip ""
	// rows of the shards
	b.Run("flood", func(b *testing.B) {
		c := NewCollector(MaxClients)
		b.ReportAllocs()
		var seq sync.Mutex
		next := uint32(0)
		b.RunParallel(func(pb *testing.PB) {
			seq.Lock()
			base := next
			next += 1 << 24
			seq.Unlock()
			i := base
			for pb.Next() {
				ip := netip.AddrFrom4([4]byte{byte(i >> 24), byte(i >> 16), byte(i >> 8), byte(i)})
				c.record(clientKey(ip), counter{requests: 1})
				i++
			}
		})
	})
}

func BenchmarkDrain(b *testing.B) {
	c := NewCollector(MaxClients)
	b.ReportAllocs()
	for b.Loop() {
		b.StopTimer()
		for i := range MaxClients {
			ip := netip.AddrFrom4([4]byte{10, byte(i >> 16), byte(i >> 8), byte(i)})
			c.record(clientKey(ip), counter{requests: int64(i), blocked: int64(i % 7)})
		}
		b.StartTimer()
		c.Drain()
	}
}
