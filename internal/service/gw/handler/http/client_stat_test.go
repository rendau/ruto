package http

import (
	"net/http"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"

	rateLimitModel "github.com/rendau/ruto/internal/domain/ratelimit/model"
	rootModel "github.com/rendau/ruto/internal/domain/root/model"
	"github.com/rendau/ruto/internal/service/gw/service/blocklist"
	"github.com/rendau/ruto/internal/service/gw/service/clientstat"
)

func TestClientStat_CountsEveryRequest(t *testing.T) {
	env := newRateLimitEnv(t, 1000, "10.0.0.0/8")
	env.blocks.Set("v1", []blocklist.Rule{{Ip: "203.0.113.0/24"}})
	conf := blockedSnapshot()
	conf.eps["a2"].Rules[0].Name = "per ip"
	conf.trusted = []rootModel.RootTrustedClient{{Name: "office", Ip: "198.51.100.0/24"}}
	h := env.handler(conf)

	preflight := http.Header{"Origin": {"https://front.example"}, "Access-Control-Request-Method": {"GET"}}
	xff := func(v string) http.Header { return http.Header{"X-Forwarded-For": {v}} }

	const blocked, limited = "203.0.113.7:1000", "192.0.2.1:1000"
	codes(h, 2, "/a/one", blocked, nil)
	doFrom(h, http.MethodOptions, "/a/one", blocked, preflight)
	doFrom(h, http.MethodGet, "/nowhere", blocked, nil)

	assert.Equal(t, []int{sOK, s429, s429}, codes(h, 3, "/a/two", limited, nil))
	assert.Equal(t, http.StatusNotFound, doFrom(h, http.MethodGet, "/nowhere", limited, nil).Code)
	doFrom(h, http.MethodOptions, "/a/one", limited, preflight)

	// a trusted client is neither blocked nor limited, and counted by its entry
	assert.Equal(t, []int{sOK, sOK}, codes(h, 2, "/a/two", "10.0.0.1:1000", xff("198.51.100.1")))
	assert.Equal(t, sOK, doFrom(h, http.MethodGet, "/a/two", "10.0.0.1:1000", xff("198.51.100.2")).Code)
	// behind the proxy, an address that cannot be told
	doFrom(h, http.MethodGet, "/a/two", "10.0.0.1:1000", xff("garbage"))
	doFrom(h, http.MethodGet, "/a/two", "[2001:db8:1:2::99]:1000", nil)

	got := lo.KeyBy(env.clients.Drain(), func(v *clientstat.Item) string { return v.Ip })
	// the block list refuses a preflight before CORS answers it
	assert.Equal(t, &clientstat.Item{Ip: "203.0.113.7", Requests: 4, Blocked: 4}, got["203.0.113.7"])
	assert.Equal(t, &clientstat.Item{Ip: "192.0.2.1", Requests: 5, Preflights: 1, Limited: 2, LastRule: "per ip"}, got["192.0.2.1"])
	assert.Equal(t, &clientstat.Item{Ip: "198.51.100.0/24", Trusted: "office", Requests: 3}, got["198.51.100.0/24"])
	assert.Equal(t, &clientstat.Item{Requests: 1}, got[""])
	assert.Equal(t, &clientstat.Item{Ip: "2001:db8:1:2::/64", Requests: 1}, got["2001:db8:1:2::/64"])
	assert.Len(t, got, 5)
}

func TestClientStat_RateLimitByPeerWhenClientUnknown(t *testing.T) {
	env := newRateLimitEnv(t, 1000, "10.0.0.0/8")
	h := env.handler(&rateLimitSnapshot{eps: map[string]rateLimitModel.RateLimit{
		"a1": {Rules: []*rateLimitModel.Rule{ipRule(1)}},
	}})

	garbage := http.Header{"X-Forwarded-For": {"garbage"}}
	assert.Equal(t, sOK, doFrom(h, http.MethodGet, "/a/one", "10.0.0.1:1000", garbage).Code)
	assert.Equal(t, s429, doFrom(h, http.MethodGet, "/a/one", "10.0.0.1:2000", garbage).Code)
	assert.Equal(t, sOK, doFrom(h, http.MethodGet, "/a/one", "10.0.0.2:1000", garbage).Code, "another peer")

	assert.Equal(t, []*clientstat.Item{{Requests: 3, Limited: 1, LastRule: "r"}}, env.clients.Drain())
}
