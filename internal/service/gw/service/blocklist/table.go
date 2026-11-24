package blocklist

import (
	"cmp"
	"maps"
	"net/netip"
	"slices"

	rootModel "github.com/rendau/ruto/internal/domain/root/model"
)

// Rule is a block as core gives it: a network in CIDR notation and when the
// block ends, unix seconds, 0 = without end.
type Rule struct {
	Ip            string
	ExpiresAtUnix int64
}

// table is an immutable block list. Networks are grouped by family and prefix
// length, one map per length keyed by the masked address, so a lookup costs a
// map access per distinct length, not per rule: a flood of /32 blocks is still
// one access for every IPv4 client.
type table struct {
	version string
	size    int
	v4      []level
	v6      []level
}

type level struct {
	bits int
	// masked address → expires at, unix seconds, 0 = without end
	nets map[netip.Addr]int64
}

// buildTable drops the rules that already ended at now and returns the values
// that are not an address or a network.
func buildTable(version string, rules []Rule, now int64) (*table, []string) {
	var invalid []string
	v4 := make(map[int]map[netip.Addr]int64)
	v6 := make(map[int]map[netip.Addr]int64)
	size := 0

	for _, r := range rules {
		prefix, err := rootModel.ParseClientPrefix(r.Ip)
		if err != nil {
			invalid = append(invalid, r.Ip)
			continue
		}
		if expired(r.ExpiresAtUnix, now) {
			continue
		}

		byBits := v6
		if prefix.Addr().Is4() {
			byBits = v4
		}
		nets, ok := byBits[prefix.Bits()]
		if !ok {
			nets = make(map[netip.Addr]int64)
			byBits[prefix.Bits()] = nets
		}
		addr := prefix.Addr()
		if prev, dup := nets[addr]; dup {
			// the same network twice: the longer block wins
			nets[addr] = longer(prev, r.ExpiresAtUnix)
			continue
		}
		nets[addr] = r.ExpiresAtUnix
		size++
	}

	return &table{
		version: version,
		size:    size,
		v4:      levels(v4),
		v6:      levels(v6),
	}, invalid
}

// levels go longest prefix first, so a lookup reports the most specific rule.
func levels(byBits map[int]map[netip.Addr]int64) []level {
	result := make([]level, 0, len(byBits))
	for _, bits := range slices.SortedFunc(maps.Keys(byBits), func(a, b int) int { return cmp.Compare(b, a) }) {
		result = append(result, level{bits: bits, nets: byBits[bits]})
	}
	return result
}

// lookup returns the active rule covering ip.
func (t *table) lookup(ip netip.Addr, now int64) (netip.Prefix, bool) {
	ip = ip.Unmap().WithZone("")
	levels := t.v6
	if ip.Is4() {
		levels = t.v4
	}

	for _, lv := range levels {
		prefix, err := ip.Prefix(lv.bits)
		if err != nil {
			continue
		}
		if expiresAt, ok := lv.nets[prefix.Addr()]; ok && !expired(expiresAt, now) {
			return prefix, true
		}
	}

	return netip.Prefix{}, false
}

func expired(expiresAt, now int64) bool {
	return expiresAt != 0 && expiresAt <= now
}

func longer(a, b int64) int64 {
	if a == 0 || b == 0 {
		return 0
	}
	return max(a, b)
}
