package blocklist

import (
	"fmt"
	"net/http"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	rootModel "github.com/rendau/ruto/internal/domain/root/model"
	"github.com/rendau/ruto/internal/service/gw/service/clientip"
)

func newTestService(now time.Time, trustedProxies ...string) (*Service, *time.Time) {
	clock := new(now)
	s := New(clientip.New(trustedProxies, nil))
	s.now = func() time.Time { return *clock }
	return s, clock
}

func matched(t *testing.T, tb *table, ip string, now int64) string {
	t.Helper()
	prefix, ok := tb.lookup(netip.MustParseAddr(ip), now)
	if !ok {
		return ""
	}
	return prefix.String()
}

func TestTable_Lookup(t *testing.T) {
	tb, invalid := buildTable("v1", []Rule{
		{Ip: "203.0.113.7/32"},
		{Ip: "198.51.100.0/24"},
		{Ip: "10.0.0.0/8"},
		{Ip: "10.1.2.3"},
		{Ip: "2001:db8:1:2::/64"},
		{Ip: "2001:db8::1"},
		{Ip: "nope"},
		{Ip: ""},
	}, 1000)

	assert.Equal(t, []string{"nope", ""}, invalid)
	assert.Equal(t, 6, tb.size)
	assert.Equal(t, "v1", tb.version)

	assert.Equal(t, "203.0.113.7/32", matched(t, tb, "203.0.113.7", 1000))
	assert.Empty(t, matched(t, tb, "203.0.113.8", 1000))
	assert.Equal(t, "198.51.100.0/24", matched(t, tb, "198.51.100.200", 1000))
	assert.Empty(t, matched(t, tb, "198.51.101.1", 1000))
	// the most specific rule is reported
	assert.Equal(t, "10.1.2.3/32", matched(t, tb, "10.1.2.3", 1000))
	assert.Equal(t, "10.0.0.0/8", matched(t, tb, "10.200.0.1", 1000))
	// a client in the blocked /64 whatever its interface id
	assert.Equal(t, "2001:db8:1:2::/64", matched(t, tb, "2001:db8:1:2:aaaa:bbbb:cccc:dddd", 1000))
	assert.Empty(t, matched(t, tb, "2001:db8:1:3::1", 1000))
	assert.Equal(t, "2001:db8::1/128", matched(t, tb, "2001:db8::1", 1000))
	assert.Empty(t, matched(t, tb, "2001:db8::2", 1000))
	// an IPv4-mapped client is the IPv4 one, a zone does not matter
	assert.Equal(t, "203.0.113.7/32", matched(t, tb, "::ffff:203.0.113.7", 1000))
	assert.Equal(t, "2001:db8::1/128", matched(t, tb, "2001:db8::1%eth0", 1000))
	// families do not mix: ::/8-like IPv6 never matches an IPv4 rule
	assert.Empty(t, matched(t, tb, "::a00:1", 1000))
}

func TestTable_Expiry(t *testing.T) {
	tb, _ := buildTable("v1", []Rule{
		{Ip: "203.0.113.1", ExpiresAtUnix: 900},  // ended before the fetch
		{Ip: "203.0.113.2", ExpiresAtUnix: 2000}, // ends later
		{Ip: "203.0.113.3"},                      // without end
		// the same network twice: the longer block wins
		{Ip: "203.0.113.4", ExpiresAtUnix: 1500},
		{Ip: "203.0.113.4/32", ExpiresAtUnix: 2500},
		{Ip: "203.0.113.5", ExpiresAtUnix: 1500},
		{Ip: "203.0.113.5"},
		// an ended specific rule does not hide an active wider one
		{Ip: "198.51.100.7", ExpiresAtUnix: 1500},
		{Ip: "198.51.100.0/24"},
	}, 1000)

	assert.Equal(t, 6, tb.size)
	assert.Empty(t, matched(t, tb, "203.0.113.1", 1000))
	assert.NotEmpty(t, matched(t, tb, "203.0.113.2", 1999))
	assert.Empty(t, matched(t, tb, "203.0.113.2", 2000), "ignored locally once past expires_at")
	assert.NotEmpty(t, matched(t, tb, "203.0.113.3", 1<<40))
	assert.NotEmpty(t, matched(t, tb, "203.0.113.4", 2400))
	assert.Empty(t, matched(t, tb, "203.0.113.4", 2500))
	assert.NotEmpty(t, matched(t, tb, "203.0.113.5", 1<<40))
	assert.Equal(t, "198.51.100.7/32", matched(t, tb, "198.51.100.7", 1000))
	assert.Equal(t, "198.51.100.0/24", matched(t, tb, "198.51.100.7", 1600))
}

func TestTable_ManyLengths(t *testing.T) {
	rules := make([]Rule, 0, 10_000)
	for i := range 9_000 {
		rules = append(rules, Rule{Ip: fmt.Sprintf("100.%d.%d.%d", i>>16&0xff, i>>8&0xff, i&0xff)})
	}
	for bits := 8; bits <= 31; bits++ {
		rules = append(rules, Rule{Ip: fmt.Sprintf("%d.0.0.0/%d", bits+150, bits)})
	}
	for i := range 976 {
		rules = append(rules, Rule{Ip: fmt.Sprintf("2001:db8:%x::/%d", i, 48+i%17)})
	}

	tb, invalid := buildTable("v", rules, 0)
	require.Empty(t, invalid)
	assert.Len(t, tb.v4, 25) // 8…32
	assert.Len(t, tb.v6, 17) // 48…64
	assert.Equal(t, "100.0.35.39/32", matched(t, tb, "100.0.35.39", 0))
	assert.Empty(t, matched(t, tb, "100.0.35.40", 0))
	assert.Equal(t, "158.0.0.0/8", matched(t, tb, "158.200.1.1", 0))
	assert.Equal(t, "181.0.0.0/31", matched(t, tb, "181.0.0.1", 0))
	assert.Empty(t, matched(t, tb, "181.0.0.2", 0))
	assert.Equal(t, "2001:db8:10::/64", matched(t, tb, "2001:db8:10::5", 0))
	assert.Empty(t, matched(t, tb, "2001:db9::1", 0))
}

func TestService_SetAndVersion(t *testing.T) {
	s, _ := newTestService(time.Unix(1000, 0))
	guard := s.Guard(nil, ProtocolHTTP)

	assert.False(t, s.Applied())
	assert.Empty(t, s.Version())
	assert.False(t, guard.Active())
	assert.False(t, guard.Blocked("203.0.113.7:1000", nil), "nobody is blocked before the first list")

	s.Set("v1", []Rule{{Ip: "203.0.113.7"}})
	assert.True(t, s.Applied())
	assert.Equal(t, "v1", s.Version())
	assert.Equal(t, 1, s.Len())
	assert.True(t, guard.Active())
	assert.True(t, guard.Blocked("203.0.113.7:1000", nil), "a guard built earlier sees the new list")

	s.Set("v2", nil)
	assert.True(t, s.Applied())
	assert.Equal(t, "v2", s.Version())
	assert.False(t, guard.Active())
	assert.False(t, guard.Blocked("203.0.113.7:1000", nil))

	var none *Service
	assert.Nil(t, none.Guard(nil, ProtocolHTTP))
	var noGuard *Guard
	assert.False(t, noGuard.Active())
	assert.False(t, noGuard.Blocked("203.0.113.7:1000", nil))
}

func TestGuard_ExpiresAtRequestTime(t *testing.T) {
	s, clock := newTestService(time.Unix(1000, 0))
	s.Set("v1", []Rule{{Ip: "203.0.113.7", ExpiresAtUnix: 1060}})
	guard := s.Guard(nil, ProtocolHTTP)

	assert.True(t, guard.Blocked("203.0.113.7:1000", nil))
	*clock = time.Unix(1060, 0)
	assert.False(t, guard.Blocked("203.0.113.7:1000", nil))
}

func TestGuard_ClientAddress(t *testing.T) {
	s, _ := newTestService(time.Unix(1000, 0), "10.0.0.0/8")
	s.Set("v1", []Rule{{Ip: "203.0.113.7"}})
	guard := s.Guard(nil, ProtocolGRPC)

	xff := func(v string) http.Header { return http.Header{"X-Forwarded-For": {v}} }

	assert.True(t, guard.Blocked("203.0.113.7:1000", nil))
	assert.True(t, guard.Blocked("[::ffff:203.0.113.7]:1000", nil))
	// headers of an untrusted peer are not read: no dodging, no framing
	assert.True(t, guard.Blocked("203.0.113.7:1000", xff("198.51.100.1")))
	assert.True(t, guard.Blocked("203.0.113.7:1000", http.Header{"X-Real-Ip": {"198.51.100.1"}}))
	assert.False(t, guard.Blocked("198.51.100.1:1000", xff("203.0.113.7")))
	// behind a trusted proxy the forwarded client counts
	assert.True(t, guard.Blocked("10.0.0.1:1000", xff("198.51.100.1, 203.0.113.7")))
	assert.True(t, guard.Blocked("10.0.0.1:1000", http.Header{"x-forwarded-for": {"203.0.113.7"}}), "grpc metadata keys are lowercase")
	assert.False(t, guard.Blocked("10.0.0.1:1000", xff("203.0.113.7, 198.51.100.1")), "only the rightmost untrusted address is the client")
	// a client that cannot be told is not blocked
	assert.False(t, guard.Blocked("10.0.0.1:1000", xff("garbage")))
	assert.False(t, guard.Blocked("", nil))
}

func TestGuard_TrustedWins(t *testing.T) {
	s, _ := newTestService(time.Unix(1000, 0))
	s.Set("v1", []Rule{{Ip: "203.0.113.0/24"}})

	snapshot := rootModel.NewEmpty()
	snapshot.TrustedClients = []rootModel.RootTrustedClient{{Name: "office", Ip: "203.0.113.7"}}
	require.NoError(t, snapshot.Normalize())
	guard := s.Guard(snapshot, ProtocolHTTP)

	assert.False(t, guard.Blocked("203.0.113.7:1000", nil))
	assert.True(t, guard.Blocked("203.0.113.8:1000", nil))
}

func BenchmarkGuard_Blocked(b *testing.B) {
	s, _ := newTestService(time.Now())
	rules := make([]Rule, 0, 10_000)
	for i := range 10_000 {
		rules = append(rules, Rule{Ip: fmt.Sprintf("100.%d.%d.%d/%d", i>>16&0xff, i>>8&0xff, i&0xff, 24+i%9)})
	}
	s.Set("v", rules)
	guard := s.Guard(nil, ProtocolHTTP)

	for b.Loop() {
		guard.Blocked("203.0.113.7:1000", nil)
	}
}
