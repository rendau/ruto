// Package clientstat counts the requests of every client of the gateway: how
// many it sent, how many the block list refused and how many rate limits
// rejected. The counts go to core with the heartbeat, for the admin to see who
// calls the gateway and who is held back.
package clientstat

import (
	"cmp"
	"hash/maphash"
	"net/netip"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
)

const (
	// MaxClients caps distinct clients kept between two drains: an attack
	// from many addresses must not grow the collector without bound. A client
	// above the cap is counted in the ip "" row, so totals stay exact.
	MaxClients = 50_000

	// ReportTop is how many clients the report names by requests, and again
	// by refusals (blocked + limited); the rest is summed into the ip "" row.
	ReportTop = 300

	// shardCount spreads the clients over independent locks, so concurrent
	// requests rarely wait for each other. A power of two.
	shardCount = 64
)

// Item is one row of the report.
type Item struct {
	// Ip is the client address, an IPv6 client as its /64; for a trusted
	// client the trusted network. "" sums the clients that did not fit and
	// the ones whose address could not be told.
	Ip string
	// Trusted is the name of the trusted client, "" for any other.
	Trusted  string
	Requests int64
	Blocked  int64
	Limited  int64
	// LastRule is the label of the rate-limit rule of the latest rejection.
	LastRule string
}

// key identifies a client without formatting its address on the request
// path; the zero key is the ip "" row.
type key struct {
	prefix  netip.Prefix
	trusted string
}

type counter struct {
	requests int64
	blocked  int64
	limited  int64
	lastRule string
	// ruleAt orders the rejections of the rows summed together, so the sum
	// keeps the rule of the latest one
	ruleAt int64
}

type shard struct {
	mu    sync.Mutex
	items map[key]counter
	// keeps neighbouring shards off one cache line
	_ [48]byte
}

// Collector holds the counts between two drains. The request path takes one
// short lock of one shard per update and never waits for a drain longer than
// the swap of that shard's map.
type Collector struct {
	seed   maphash.Seed
	max    int64
	size   atomic.Int64
	shards [shardCount]shard
}

var ins = NewCollector(MaxClients)

// Ins is the process-wide collector: it outlives handler rebuilds.
func Ins() *Collector {
	return ins
}

// NewCollector keeps at most maxClients distinct clients between drains;
// trusted clients and the ip "" row are not counted against it.
func NewCollector(maxClients int) *Collector {
	c := &Collector{
		seed: maphash.MakeSeed(),
		max:  int64(maxClients),
	}
	for i := range c.shards {
		c.shards[i].items = make(map[key]counter)
	}
	return c
}

func (c *Collector) record(k key, d counter) {
	addr := k.prefix.Addr().As16()
	s := &c.shards[maphash.Bytes(c.seed, addr[:])&(shardCount-1)]

	s.mu.Lock()
	v, ok := s.items[k]
	if !ok && k.capped() {
		if c.size.Load() >= c.max {
			k = key{}
			v = s.items[k]
		} else {
			c.size.Add(1)
		}
	}
	v.merge(&d)
	s.items[k] = v
	s.mu.Unlock()
}

// Drain returns the counts since the previous drain and starts from scratch.
// The report is bounded: every trusted client, the ReportTop clients by
// requests and the ReportTop by refusals; all the others are summed into one
// ip "" row.
func (c *Collector) Drain() []*Item {
	type entry struct {
		key
		counter
		named bool
	}

	var entries []entry
	var rest counter
	var trusted []*Item

	for i := range c.shards {
		s := &c.shards[i]
		s.mu.Lock()
		items := s.items
		s.items = make(map[key]counter)
		s.mu.Unlock()

		var capped int64
		for k, v := range items {
			switch {
			case !k.prefix.IsValid():
				rest.merge(&v)
			case k.trusted != "":
				trusted = append(trusted, k.item(&v))
			default:
				capped++
				entries = append(entries, entry{key: k, counter: v})
			}
		}
		c.size.Add(-capped)
	}

	slices.SortFunc(entries, func(a, b entry) int { return cmp.Compare(b.requests, a.requests) })
	for i := range min(len(entries), ReportTop) {
		entries[i].named = true
	}
	slices.SortFunc(entries, func(a, b entry) int {
		return cmp.Compare(b.blocked+b.limited, a.blocked+a.limited)
	})
	for i := range min(len(entries), ReportTop) {
		if entries[i].blocked+entries[i].limited > 0 {
			entries[i].named = true
		}
	}

	named := make([]*Item, 0, min(len(entries), 2*ReportTop))
	for i := range entries {
		if entries[i].named {
			named = append(named, entries[i].key.item(&entries[i].counter))
		} else {
			rest.merge(&entries[i].counter)
		}
	}
	slices.SortFunc(named, func(a, b *Item) int { return cmp.Compare(b.Requests, a.Requests) })
	slices.SortFunc(trusted, func(a, b *Item) int { return strings.Compare(a.Ip, b.Ip) })

	result := append(trusted, named...)
	if rest.requests+rest.blocked+rest.limited > 0 {
		result = append(result, (&key{}).item(&rest))
	}
	return result
}

// Restore puts a drained report back, e.g. when it could not be delivered.
// Counts made since the drain keep their rule: they are the later ones.
func (c *Collector) Restore(items []*Item) {
	for _, item := range items {
		c.record(parseKey(item.Ip, item.Trusted), counter{
			requests: item.Requests,
			blocked:  item.Blocked,
			limited:  item.Limited,
			lastRule: item.LastRule,
		})
	}
}

func (v *counter) merge(d *counter) {
	v.requests += d.requests
	v.blocked += d.blocked
	v.limited += d.limited
	if d.lastRule != "" && (v.lastRule == "" || d.ruleAt >= v.ruleAt) {
		v.lastRule = d.lastRule
		v.ruleAt = d.ruleAt
	}
}

// capped reports whether the key counts against MaxClients: a trusted client
// is bounded by the configuration, the ip "" row is the overflow itself.
func (k *key) capped() bool {
	return k.prefix.IsValid() && k.trusted == ""
}

func (k *key) ip() string {
	if !k.prefix.IsValid() {
		return ""
	}
	if k.trusted == "" && k.prefix.IsSingleIP() {
		return k.prefix.Addr().String()
	}
	return k.prefix.String()
}

func (k *key) item(v *counter) *Item {
	return &Item{
		Ip:       k.ip(),
		Trusted:  k.trusted,
		Requests: v.requests,
		Blocked:  v.blocked,
		Limited:  v.limited,
		LastRule: v.lastRule,
	}
}

// clientKey is the row of a client address: IPv4 as is, IPv6 as its /64, the
// smallest network a provider hands out (rate limits count it the same way).
func clientKey(ip netip.Addr) key {
	if !ip.IsValid() {
		return key{}
	}
	ip = ip.Unmap().WithZone("")
	if ip.Is4() {
		return key{prefix: netip.PrefixFrom(ip, 32)}
	}
	prefix, err := ip.Prefix(64)
	if err != nil {
		return key{}
	}
	return key{prefix: prefix}
}

// parseKey reverses key.ip; anything unreadable lands in the ip "" row.
func parseKey(ip, trusted string) key {
	if ip == "" {
		return key{}
	}
	if strings.Contains(ip, "/") {
		prefix, err := netip.ParsePrefix(ip)
		if err != nil {
			return key{}
		}
		return key{prefix: prefix.Masked(), trusted: trusted}
	}
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return key{}
	}
	if trusted != "" {
		return key{prefix: netip.PrefixFrom(addr, addr.BitLen()), trusted: trusted}
	}
	return clientKey(addr)
}
