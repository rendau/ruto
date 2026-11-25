package http

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appModel "github.com/rendau/ruto/internal/domain/app/model"
	authModel "github.com/rendau/ruto/internal/domain/auth/model"
	endpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
	rateLimitModel "github.com/rendau/ruto/internal/domain/ratelimit/model"
	rootModel "github.com/rendau/ruto/internal/domain/root/model"
	"github.com/rendau/ruto/internal/infra/clientip"
	"github.com/rendau/ruto/internal/service/gw/handler/http/proxy"
	"github.com/rendau/ruto/internal/service/gw/service/blocklist"
	"github.com/rendau/ruto/internal/service/gw/service/clientstat"
	"github.com/rendau/ruto/internal/service/gw/service/ratelimit"
)

func ipRule(requests int) *rateLimitModel.Rule {
	return &rateLimitModel.Rule{Name: "r", Key: "ip", Requests: requests, Period: "1m"}
}

func rateLimitEndpoint(id, path string, rl rateLimitModel.RateLimit) *endpointModel.Endpoint {
	return &endpointModel.Endpoint{
		Id:        id,
		Active:    true,
		Type:      endpointModel.TypeHTTP,
		Http:      endpointModel.Http{Method: http.MethodGet, Path: path},
		RateLimit: rl,
	}
}

// rateLimitSnapshot gives a fresh snapshot each call, as the gateway gets one
// on every config change: two apps, /a (endpoints a1, a2) and /b (b1).
type rateLimitSnapshot struct {
	root    rateLimitModel.RateLimit
	app     rateLimitModel.RateLimit
	eps     map[string]rateLimitModel.RateLimit
	cors    bool
	auth    *authModel.Auth
	trusted []rootModel.RootTrustedClient
}

func (c *rateLimitSnapshot) build(t *testing.T, backendURL string) *rootModel.Root {
	t.Helper()

	clone := func(rl rateLimitModel.RateLimit) rateLimitModel.RateLimit {
		return rateLimitModel.RateLimit{Mode: rl.Mode, Rules: rl.CloneRules()}
	}

	a1 := rateLimitEndpoint("a1", "one", clone(c.eps["a1"]))
	if c.auth != nil {
		a1.Auth = *c.auth
	}

	snapshot := rootModel.NewEmpty()
	snapshot.Cors.Enabled = c.cors
	snapshot.TrustedClients = slices.Clone(c.trusted)
	snapshot.RateLimit = clone(c.root)
	snapshot.Apps = []*appModel.App{
		{
			Id:         "app-a",
			Active:     true,
			PathPrefix: "/a",
			Name:       "a",
			Backend:    appModel.Backend{Url: backendURL},
			RateLimit:  clone(c.app),
			Endpoints: []*endpointModel.Endpoint{
				a1,
				rateLimitEndpoint("a2", "two", clone(c.eps["a2"])),
			},
		},
		{
			Id:         "app-b",
			Active:     true,
			PathPrefix: "/b",
			Name:       "b",
			Backend:    appModel.Backend{Url: backendURL},
			Endpoints: []*endpointModel.Endpoint{
				rateLimitEndpoint("b1", "one", clone(c.eps["b1"])),
			},
		},
	}
	require.NoError(t, snapshot.Normalize())
	snapshot.InheritDown()
	snapshot.Interpolate()

	return snapshot
}

type rateLimitEnv struct {
	t       *testing.T
	backend *httptest.Server
	store   *ratelimit.Store
	limits  *ratelimit.Service
	blocks  *blocklist.Service
	clients *clientstat.Collector
	stats   *clientstat.Service
}

func newRateLimitEnv(t *testing.T, maxKeys int, trustedProxies ...string) *rateLimitEnv {
	t.Helper()

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(backend.Close)

	store := ratelimit.New(maxKeys)
	clientIP := clientip.New(trustedProxies, nil)
	clients := clientstat.NewCollector(clientstat.MaxClients)
	return &rateLimitEnv{
		t:       t,
		backend: backend,
		store:   store,
		limits:  ratelimit.NewService(store, clientIP),
		blocks:  blocklist.New(clientIP),
		clients: clients,
		stats:   clientstat.New(clients, clientIP),
	}
}

func (e *rateLimitEnv) handler(conf *rateLimitSnapshot) *Service {
	e.t.Helper()

	service, err := New(conf.build(e.t, e.backend.URL), proxy.NewTransport(0), nil, e.limits, e.blocks, e.stats, nil)
	require.NoError(e.t, err)
	return service
}

func doFrom(h http.Handler, method, path, remoteAddr string, headers http.Header) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	req.RemoteAddr = remoteAddr
	for key, values := range headers {
		req.Header[key] = values
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// codes sends n GET requests and returns the response codes.
func codes(h http.Handler, n int, path, remoteAddr string, headers http.Header) []int {
	result := make([]int, n)
	for i := range n {
		result[i] = doFrom(h, http.MethodGet, path, remoteAddr, headers).Code
	}
	return result
}

const (
	sOK  = http.StatusOK
	s429 = http.StatusTooManyRequests
)

func TestRateLimit_RejectsOverLimit(t *testing.T) {
	env := newRateLimitEnv(t, 1000)
	h := env.handler(&rateLimitSnapshot{eps: map[string]rateLimitModel.RateLimit{
		"a1": {Rules: []*rateLimitModel.Rule{ipRule(3)}},
	}})

	assert.Equal(t, []int{sOK, sOK, sOK}, codes(h, 3, "/a/one", "203.0.113.7:1000", nil))

	rec := doFrom(h, http.MethodGet, "/a/one", "203.0.113.7:2000", nil)
	require.Equal(t, s429, rec.Code)
	retryAfter, err := strconv.Atoi(rec.Header().Get("Retry-After"))
	require.NoError(t, err)
	assert.GreaterOrEqual(t, retryAfter, 1)
	assert.LessOrEqual(t, retryAfter, 60)
	assert.Equal(t, "Too Many Requests\n", rec.Body.String())
	assert.Equal(t, "text/plain; charset=utf-8", rec.Header().Get("Content-Type"))

	// another client and an endpoint without rules are not affected
	assert.Equal(t, sOK, doFrom(h, http.MethodGet, "/a/one", "203.0.113.8:1000", nil).Code)
	assert.Equal(t, []int{sOK, sOK, sOK, sOK, sOK}, codes(h, 5, "/a/two", "203.0.113.7:1000", nil))
}

func TestRateLimit_RootRuleSharedAcrossApps(t *testing.T) {
	env := newRateLimitEnv(t, 1000)
	h := env.handler(&rateLimitSnapshot{root: rateLimitModel.RateLimit{Rules: []*rateLimitModel.Rule{ipRule(3)}}})

	const client = "203.0.113.7:1000"
	assert.Equal(t, []int{sOK, sOK}, codes(h, 2, "/a/one", client, nil))
	assert.Equal(t, sOK, doFrom(h, http.MethodGet, "/b/one", client, nil).Code)

	assert.Equal(t, s429, doFrom(h, http.MethodGet, "/a/two", client, nil).Code)
	assert.Equal(t, s429, doFrom(h, http.MethodGet, "/b/one", client, nil).Code)
}

func TestRateLimit_AppRuleSharedWithinApp(t *testing.T) {
	env := newRateLimitEnv(t, 1000)
	h := env.handler(&rateLimitSnapshot{app: rateLimitModel.RateLimit{Rules: []*rateLimitModel.Rule{ipRule(2)}}})

	const client = "203.0.113.7:1000"
	assert.Equal(t, sOK, doFrom(h, http.MethodGet, "/a/one", client, nil).Code)
	assert.Equal(t, sOK, doFrom(h, http.MethodGet, "/a/two", client, nil).Code)
	assert.Equal(t, s429, doFrom(h, http.MethodGet, "/a/one", client, nil).Code)
	// app b has no rules
	assert.Equal(t, []int{sOK, sOK, sOK}, codes(h, 3, "/b/one", client, nil))
}

func TestRateLimit_EndpointRuleIndependentPerEndpoint(t *testing.T) {
	env := newRateLimitEnv(t, 1000)
	// the very same rule on two endpoints
	h := env.handler(&rateLimitSnapshot{eps: map[string]rateLimitModel.RateLimit{
		"a1": {Rules: []*rateLimitModel.Rule{ipRule(2)}},
		"a2": {Rules: []*rateLimitModel.Rule{ipRule(2)}},
	}})

	const client = "203.0.113.7:1000"
	assert.Equal(t, []int{sOK, sOK, s429}, codes(h, 3, "/a/one", client, nil))
	assert.Equal(t, []int{sOK, sOK, s429}, codes(h, 3, "/a/two", client, nil))
}

func TestRateLimit_ReplaceWithNoRulesIsUnlimited(t *testing.T) {
	env := newRateLimitEnv(t, 1000)
	h := env.handler(&rateLimitSnapshot{
		root: rateLimitModel.RateLimit{Rules: []*rateLimitModel.Rule{ipRule(1)}},
		eps:  map[string]rateLimitModel.RateLimit{"a2": {Mode: "replace"}},
	})

	const client = "203.0.113.7:1000"
	assert.Equal(t, []int{sOK, sOK, sOK, sOK, sOK}, codes(h, 5, "/a/two", client, nil))
	assert.Equal(t, []int{sOK, s429}, codes(h, 2, "/a/one", client, nil))
}

func TestRateLimit_IPv6CountsByNetwork(t *testing.T) {
	env := newRateLimitEnv(t, 1000)
	h := env.handler(&rateLimitSnapshot{eps: map[string]rateLimitModel.RateLimit{
		"a1": {Rules: []*rateLimitModel.Rule{ipRule(2)}},
	}})

	assert.Equal(t, sOK, doFrom(h, http.MethodGet, "/a/one", "[2001:db8:1:2::1]:1000", nil).Code)
	assert.Equal(t, sOK, doFrom(h, http.MethodGet, "/a/one", "[2001:db8:1:2:aaaa:bbbb:cccc:dddd]:1000", nil).Code)
	assert.Equal(t, s429, doFrom(h, http.MethodGet, "/a/one", "[2001:db8:1:2::ffff]:1000", nil).Code)

	// the neighbouring /64 is another client
	assert.Equal(t, sOK, doFrom(h, http.MethodGet, "/a/one", "[2001:db8:1:3::1]:1000", nil).Code)
}

func TestRateLimit_TokenKey(t *testing.T) {
	env := newRateLimitEnv(t, 1000)
	h := env.handler(&rateLimitSnapshot{eps: map[string]rateLimitModel.RateLimit{
		"a1": {Rules: []*rateLimitModel.Rule{{Key: "token", Requests: 2, Period: "1m"}}},
	}})

	tokenA := http.Header{"Authorization": {"Bearer aaa"}}
	tokenB := http.Header{"Authorization": {"Bearer bbb"}}

	assert.Equal(t, []int{sOK, sOK, s429}, codes(h, 3, "/a/one", "203.0.113.7:1000", tokenA))
	// the same token from another address shares the counter
	assert.Equal(t, s429, doFrom(h, http.MethodGet, "/a/one", "198.51.100.1:1000", tokenA).Code)
	// another token from the same address has its own
	assert.Equal(t, []int{sOK, sOK}, codes(h, 2, "/a/one", "203.0.113.7:1000", tokenB))

	// without a token the address counts, apart from the tokens sent from it
	assert.Equal(t, []int{sOK, sOK, s429}, codes(h, 3, "/a/one", "203.0.113.7:1000", nil))
	assert.Equal(t, sOK, doFrom(h, http.MethodGet, "/a/one", "203.0.113.8:1000", nil).Code)
}

func TestRateLimit_ForwardedFor(t *testing.T) {
	env := newRateLimitEnv(t, 1000, "10.0.0.0/8")
	h := env.handler(&rateLimitSnapshot{eps: map[string]rateLimitModel.RateLimit{
		"a1": {Rules: []*rateLimitModel.Rule{ipRule(2)}},
	}})

	xff := func(v string) http.Header { return http.Header{"X-Forwarded-For": {v}} }

	// an untrusted peer cannot pick its address by a header
	assert.Equal(t, sOK, doFrom(h, http.MethodGet, "/a/one", "203.0.113.7:1000", xff("198.51.100.1")).Code)
	assert.Equal(t, sOK, doFrom(h, http.MethodGet, "/a/one", "203.0.113.7:1000", xff("198.51.100.2")).Code)
	assert.Equal(t, s429, doFrom(h, http.MethodGet, "/a/one", "203.0.113.7:1000", xff("198.51.100.3")).Code)

	// behind a trusted proxy every client counts on its own, by the rightmost
	// untrusted address
	assert.Equal(t, []int{sOK, sOK, s429}, codes(h, 3, "/a/one", "10.0.0.1:1000", xff("1.1.1.1, 198.51.100.1, 10.0.0.2")))
	assert.Equal(t, []int{sOK, sOK}, codes(h, 2, "/a/one", "10.0.0.1:1000", xff("198.51.100.2")))
}

func TestRateLimit_CountersSurviveHandlerRebuild(t *testing.T) {
	env := newRateLimitEnv(t, 1000)
	conf := &rateLimitSnapshot{
		root: rateLimitModel.RateLimit{Rules: []*rateLimitModel.Rule{ipRule(3)}},
		eps:  map[string]rateLimitModel.RateLimit{"a1": {Rules: []*rateLimitModel.Rule{ipRule(1)}}},
	}

	const client = "203.0.113.7:1000"
	assert.Equal(t, []int{sOK, sOK}, codes(env.handler(conf), 2, "/a/two", client, nil))
	assert.Equal(t, []int{sOK, s429}, codes(env.handler(conf), 2, "/a/one", client, nil))

	h := env.handler(conf)
	assert.Equal(t, s429, doFrom(h, http.MethodGet, "/a/two", client, nil).Code)

	// an edited rule starts from zero
	conf.root.Rules[0].Requests = 4
	assert.Equal(t, sOK, doFrom(env.handler(conf), http.MethodGet, "/a/two", client, nil).Code)
}

func TestRateLimit_KeyCap(t *testing.T) {
	env := newRateLimitEnv(t, 2)
	h := env.handler(&rateLimitSnapshot{eps: map[string]rateLimitModel.RateLimit{
		"a1": {Rules: []*rateLimitModel.Rule{ipRule(2)}},
	}})

	// a throttled client
	assert.Equal(t, []int{sOK, sOK, s429}, codes(h, 3, "/a/one", "203.0.113.7:1000", nil))

	// a flood of new addresses still passes, without growing the table
	for i := 1; i <= 50; i++ {
		rec := doFrom(h, http.MethodGet, "/a/one", "198.51.100."+strconv.Itoa(i)+":1000", nil)
		require.Equal(t, sOK, rec.Code)
		require.LessOrEqual(t, env.store.Len(), 2)
	}

	assert.Equal(t, s429, doFrom(h, http.MethodGet, "/a/one", "203.0.113.7:1000", nil).Code)
}

func TestRateLimit_CorsPreflightIsFree(t *testing.T) {
	env := newRateLimitEnv(t, 1000)
	h := env.handler(&rateLimitSnapshot{
		cors: true,
		eps:  map[string]rateLimitModel.RateLimit{"a1": {Rules: []*rateLimitModel.Rule{ipRule(2)}}},
	})

	const client = "203.0.113.7:1000"
	preflight := http.Header{"Origin": {"https://front.example"}, "Access-Control-Request-Method": {"GET"}}
	for range 5 {
		assert.Equal(t, http.StatusNoContent, doFrom(h, http.MethodOptions, "/a/one", client, preflight).Code)
	}

	assert.Equal(t, []int{sOK, sOK}, codes(h, 2, "/a/one", client, nil))
	rec := doFrom(h, http.MethodGet, "/a/one", client, http.Header{"Origin": {"https://front.example"}})
	assert.Equal(t, s429, rec.Code)
	assert.Equal(t, "*", rec.Header().Get("Access-Control-Allow-Origin"), "the browser can read the 429")
}

func TestRateLimit_CountsBeforeAuth(t *testing.T) {
	env := newRateLimitEnv(t, 1000)
	h := env.handler(&rateLimitSnapshot{
		auth: &authModel.Auth{Enabled: true, Methods: []*authModel.AuthMethod{{
			Basic: &authModel.AuthMethodBasic{Users: []authModel.AuthMethodBasicUser{{Username: "u", Password: "p"}}},
		}}},
		eps: map[string]rateLimitModel.RateLimit{"a1": {Rules: []*rateLimitModel.Rule{ipRule(2)}}},
	})

	const client = "203.0.113.7:1000"
	wrong := http.Header{"Authorization": {"Basic dTp3cm9uZw=="}} // u:wrong
	right := http.Header{"Authorization": {"Basic dTpw"}}         // u:p

	assert.Equal(t, []int{http.StatusUnauthorized, http.StatusUnauthorized}, codes(h, 2, "/a/one", client, wrong))
	assert.Equal(t, s429, doFrom(h, http.MethodGet, "/a/one", client, right).Code)
}

func TestRateLimit_TokenSpellingsShareCounter(t *testing.T) {
	env := newRateLimitEnv(t, 1000)
	h := env.handler(&rateLimitSnapshot{eps: map[string]rateLimitModel.RateLimit{
		"a1": {Rules: []*rateLimitModel.Rule{{Key: "token", Requests: 5, Period: "1m"}}},
	}})

	for i, v := range []string{"Bearer abc", "bearer abc", "abc", "Bearer  abc", "Bearer\tabc"} {
		addr := "203.0.113." + strconv.Itoa(i+1) + ":1000"
		require.Equal(t, sOK, doFrom(h, http.MethodGet, "/a/one", addr, http.Header{"Authorization": {v}}).Code, v)
	}
	assert.Equal(t, s429, doFrom(h, http.MethodGet, "/a/one", "203.0.113.9:1000", http.Header{"Authorization": {"BEARER abc"}}).Code)
}

func TestRateLimit_TokenKeyBasicCountsByUser(t *testing.T) {
	env := newRateLimitEnv(t, 1000)
	h := env.handler(&rateLimitSnapshot{eps: map[string]rateLimitModel.RateLimit{
		"a1": {Rules: []*rateLimitModel.Rule{{Key: "token", Requests: 2, Period: "1m"}}},
	}})

	basic := func(userPass string) http.Header {
		return http.Header{"Authorization": {"Basic " + base64.StdEncoding.EncodeToString([]byte(userPass))}}
	}

	// password guessing against one user from many addresses adds up
	assert.Equal(t, sOK, doFrom(h, http.MethodGet, "/a/one", "203.0.113.1:1000", basic("admin:1")).Code)
	assert.Equal(t, sOK, doFrom(h, http.MethodGet, "/a/one", "203.0.113.2:1000", basic("admin:2")).Code)
	assert.Equal(t, s429, doFrom(h, http.MethodGet, "/a/one", "203.0.113.3:1000", basic("admin:3")).Code)

	// another user has its own counter; a token equal to the user name too
	assert.Equal(t, sOK, doFrom(h, http.MethodGet, "/a/one", "203.0.113.1:1000", basic("bob:1")).Code)
	assert.Equal(t, sOK, doFrom(h, http.MethodGet, "/a/one", "203.0.113.1:1000", http.Header{"Authorization": {"Bearer admin"}}).Code)

	// unparseable credentials count by ip
	garbage := http.Header{"Authorization": {"Basic !!!"}}
	assert.Equal(t, []int{sOK, sOK, s429}, codes(h, 3, "/a/one", "198.51.100.1:1000", garbage))
	assert.Equal(t, s429, doFrom(h, http.MethodGet, "/a/one", "198.51.100.1:1000", nil).Code)
}

func TestRateLimit_SpoofedClientHeadersDoNotEvade(t *testing.T) {
	env := newRateLimitEnv(t, 1000, "10.0.0.0/8")
	h := env.handler(&rateLimitSnapshot{eps: map[string]rateLimitModel.RateLimit{
		"a1": {Rules: []*rateLimitModel.Rule{ipRule(2)}},
	}})

	headers := []string{"Cf-Connecting-Ip", "True-Client-Ip", "X-Client-Ip", "X-Cluster-Client-Ip"}
	for i := range 6 {
		// traefik appended the real client; the rest is the client's own
		spoofed := http.Header{"X-Forwarded-For": {"203.0.113.7"}, headers[i%len(headers)]: {"198.51.100." + strconv.Itoa(i)}}
		want := sOK
		if i >= 2 {
			want = s429
		}
		require.Equal(t, want, doFrom(h, http.MethodGet, "/a/one", "10.0.0.1:1000", spoofed).Code, i)
	}

	// nor can a client burn the quota of the address it names
	assert.Equal(t, sOK, doFrom(h, http.MethodGet, "/a/one", "10.0.0.1:1000", http.Header{"X-Forwarded-For": {"198.51.100.0"}}).Code)
}
