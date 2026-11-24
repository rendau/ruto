package http

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	authModel "github.com/rendau/ruto/internal/domain/auth/model"
	rateLimitModel "github.com/rendau/ruto/internal/domain/ratelimit/model"
	rootModel "github.com/rendau/ruto/internal/domain/root/model"
	"github.com/rendau/ruto/internal/service/gw/service/blocklist"
)

const s403 = http.StatusForbidden

// blockedSnapshot has everything a request could stop at before the block
// list: CORS, basic auth on /a/one and a rate limit on /a/two.
func blockedSnapshot() *rateLimitSnapshot {
	return &rateLimitSnapshot{
		cors: true,
		auth: &authModel.Auth{Enabled: true, Methods: []*authModel.AuthMethod{{
			Basic: &authModel.AuthMethodBasic{Users: []authModel.AuthMethodBasicUser{{Username: "u", Password: "p"}}},
		}}},
		eps: map[string]rateLimitModel.RateLimit{"a2": {Rules: []*rateLimitModel.Rule{ipRule(1)}}},
	}
}

func TestBlocklist_RefusedBeforeEverything(t *testing.T) {
	env := newRateLimitEnv(t, 1000)
	env.blocks.Set("v1", []blocklist.Rule{{Ip: "203.0.113.0/24"}, {Ip: "2001:db8:1:2::/64"}})
	h := env.handler(blockedSnapshot())

	const blocked = "203.0.113.7:1000"
	preflight := http.Header{"Origin": {"https://front.example"}, "Access-Control-Request-Method": {"GET"}}
	right := http.Header{"Authorization": {"Basic dTpw"}} // u:p

	rec := doFrom(h, http.MethodGet, "/a/one", blocked, right)
	assert.Equal(t, s403, rec.Code)
	assert.Equal(t, "Forbidden\n", rec.Body.String())
	assert.Equal(t, "text/plain; charset=utf-8", rec.Header().Get("Content-Type"))

	assert.Equal(t, s403, doFrom(h, http.MethodOptions, "/a/one", blocked, preflight).Code, "preflight")
	assert.Empty(t, doFrom(h, http.MethodOptions, "/a/one", blocked, preflight).Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, s403, doFrom(h, http.MethodGet, "/a/one", blocked, nil).Code, "403, not the 401 of auth")
	assert.Equal(t, []int{s403, s403, s403}, codes(h, 3, "/a/two", blocked, nil), "403, not the 429 of the rate limit")
	assert.Equal(t, s403, doFrom(h, http.MethodGet, "/nowhere", blocked, nil).Code, "unknown route")
	assert.Equal(t, s403, doFrom(h, http.MethodGet, "/a/two", "[2001:db8:1:2::99]:1000", nil).Code)

	// blocked requests did not spend the rate limit of anyone, others pass
	const other = "198.51.100.1:1000"
	assert.Equal(t, http.StatusNoContent, doFrom(h, http.MethodOptions, "/a/one", other, preflight).Code)
	assert.Equal(t, sOK, doFrom(h, http.MethodGet, "/a/one", other, right).Code)
	assert.Equal(t, http.StatusUnauthorized, doFrom(h, http.MethodGet, "/a/one", other, nil).Code)
	assert.Equal(t, []int{sOK, s429}, codes(h, 2, "/a/two", other, nil))
	assert.Equal(t, http.StatusNotFound, doFrom(h, http.MethodGet, "/nowhere", other, nil).Code)
}

func TestBlocklist_AppliesWithoutRebuild(t *testing.T) {
	env := newRateLimitEnv(t, 1000)
	h := env.handler(&rateLimitSnapshot{})

	const client = "203.0.113.7:1000"
	assert.Equal(t, sOK, doFrom(h, http.MethodGet, "/a/one", client, nil).Code)

	env.blocks.Set("v1", []blocklist.Rule{{Ip: "203.0.113.7"}})
	assert.Equal(t, s403, doFrom(h, http.MethodGet, "/a/one", client, nil).Code)

	env.blocks.Set("v2", nil)
	assert.Equal(t, sOK, doFrom(h, http.MethodGet, "/a/one", client, nil).Code)
}

func TestBlocklist_TrustedClient(t *testing.T) {
	env := newRateLimitEnv(t, 1000)
	env.blocks.Set("v1", []blocklist.Rule{{Ip: "203.0.113.0/24"}})
	conf := &rateLimitSnapshot{
		root:    rateLimitModel.RateLimit{Rules: []*rateLimitModel.Rule{ipRule(1), {Key: "all", Requests: 3, Period: "1m"}}},
		trusted: []rootModel.RootTrustedClient{{Name: "office", Ip: "203.0.113.7"}, {Name: "lan", Ip: "192.168.0.0/16"}},
	}
	h := env.handler(conf)

	// trusted wins over the block and is never limited, nor counted
	assert.Equal(t, []int{sOK, sOK, sOK, sOK, sOK}, codes(h, 5, "/a/one", "203.0.113.7:1000", nil))
	assert.Equal(t, []int{sOK, sOK, sOK, sOK, sOK}, codes(h, 5, "/b/one", "192.168.10.20:1000", nil))
	assert.Equal(t, s403, doFrom(h, http.MethodGet, "/a/one", "203.0.113.8:1000", nil).Code)
	assert.Equal(t, []int{sOK, s429}, codes(h, 2, "/a/one", "198.51.100.1:1000", nil))

	// the next snapshot without the trusted client: limits and the block apply
	conf.trusted = nil
	h = env.handler(conf)
	assert.Equal(t, s403, doFrom(h, http.MethodGet, "/a/one", "203.0.113.7:1000", nil).Code)
	assert.Equal(t, []int{sOK, s429}, codes(h, 2, "/b/one", "192.168.10.20:1000", nil))
}

func TestBlocklist_SpoofedHeaders(t *testing.T) {
	env := newRateLimitEnv(t, 1000, "10.0.0.0/8")
	env.blocks.Set("v1", []blocklist.Rule{{Ip: "203.0.113.7"}})
	h := env.handler(&rateLimitSnapshot{
		trusted: []rootModel.RootTrustedClient{{Name: "office", Ip: "198.51.100.1"}},
	})

	xff := func(v string) http.Header { return http.Header{"X-Forwarded-For": {v}} }

	// an untrusted peer cannot dodge its block, pose as a trusted client or
	// frame someone else by a header
	assert.Equal(t, s403, doFrom(h, http.MethodGet, "/a/one", "203.0.113.7:1000", xff("198.51.100.2")).Code)
	assert.Equal(t, s403, doFrom(h, http.MethodGet, "/a/one", "203.0.113.7:1000", xff("198.51.100.1")).Code)
	assert.Equal(t, s403, doFrom(h, http.MethodGet, "/a/one", "203.0.113.7:1000", http.Header{"X-Real-Ip": {"198.51.100.1"}}).Code)
	assert.Equal(t, sOK, doFrom(h, http.MethodGet, "/a/one", "198.51.100.2:1000", xff("203.0.113.7")).Code)
	assert.Equal(t, sOK, doFrom(h, http.MethodGet, "/a/one", "198.51.100.2:1000", http.Header{"True-Client-Ip": {"203.0.113.7"}}).Code)

	// behind a trusted proxy: the rightmost untrusted address is the client
	assert.Equal(t, s403, doFrom(h, http.MethodGet, "/a/one", "10.0.0.1:1000", xff("198.51.100.1, 203.0.113.7")).Code)
	assert.Equal(t, s403, doFrom(h, http.MethodGet, "/a/one", "10.0.0.1:1000", http.Header{
		"X-Forwarded-For": {"203.0.113.7"}, "Cf-Connecting-Ip": {"198.51.100.1"},
	}).Code)
	assert.Equal(t, sOK, doFrom(h, http.MethodGet, "/a/one", "10.0.0.1:1000", xff("203.0.113.7, 198.51.100.2")).Code)
	// a client that cannot be told is not blocked
	assert.Equal(t, sOK, doFrom(h, http.MethodGet, "/a/one", "10.0.0.1:1000", xff("garbage")).Code)
}
