package ratelimit

import (
	"encoding/binary"
	"hash/maphash"
	"log/slog"
	"net/http"
	"net/netip"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/samber/lo"

	"github.com/rendau/ruto/internal/constant"
	domAppModel "github.com/rendau/ruto/internal/domain/app/model"
	domEndpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
	rateLimitModel "github.com/rendau/ruto/internal/domain/ratelimit/model"
	rootModel "github.com/rendau/ruto/internal/domain/root/model"
	"github.com/rendau/ruto/internal/infra/clientip"
	authModel "github.com/rendau/ruto/internal/service/gw/service/auth/model"
	"github.com/rendau/ruto/internal/service/gw/service/clientstat"
	"github.com/rendau/ruto/internal/service/gw/service/metrics"
)

const rejectWarnEvery = 10 * time.Second

// Counter keys are tagged by kind, so that a token never lands in the counter
// of an address or of a basic-auth user.
const (
	tagIP        byte = 'i'
	tagToken     byte = 't'
	tagBasicUser byte = 'u'
)

// Service builds the limiters of endpoints. It is made once per gateway and
// shared by the handlers of every snapshot.
type Service struct {
	store    *Store
	clientIP *clientip.Resolver
}

func NewService(store *Store, clientIP *clientip.Resolver) *Service {
	return &Service{store: store, clientIP: clientIP}
}

type Request struct {
	RemoteAddr string
	// Header carries the forwarding headers (X-Forwarded-For and the like),
	// used only when RemoteAddr is a trusted proxy.
	Header http.Header
	// Authorization is the raw Authorization value for the "token" key; it is
	// parsed the way auth parses it, see credential.
	Authorization string
	// Client, when set, is the client already resolved for the request: its
	// address is used instead of resolving it again, and a rejection is
	// counted in its stats.
	Client *clientstat.Client
}

// Endpoint is the limiter of one endpoint of a snapshot.
type Endpoint struct {
	service *Service
	app     *domAppModel.App
	ep      *domEndpointModel.Endpoint
	route   string
	rules   []*rule
	// the snapshot, when it has trusted clients: they are never limited
	trusted *rootModel.Root
}

type rule struct {
	*rateLimitModel.Rule
	hash     uint64
	label    string
	rejected prometheus.Counter
	warn     throttle
}

// Endpoint returns the limiter of ep of snapshot, or nil when no rule applies
// to it.
func (s *Service) Endpoint(snapshot *rootModel.Root, app *domAppModel.App, ep *domEndpointModel.Endpoint, route string) *Endpoint {
	if s == nil || len(ep.RateLimit.Rules) == 0 {
		return nil
	}

	rules := lo.Map(ep.RateLimit.Rules, func(v *rateLimitModel.Rule, _ int) *rule {
		label := ruleLabel(v)
		return &rule{
			Rule:     v,
			hash:     maphash.String(s.store.seed, ruleIdentity(v)),
			label:    label,
			rejected: metrics.RateLimitRejected(app, ep, v.Scope, label),
			warn:     throttle{every: rejectWarnEvery},
		}
	})
	// The same rule written twice on a level shares one counter: counting it
	// twice would halve its limit.
	rules = lo.UniqBy(rules, func(v *rule) uint64 { return v.hash })

	result := &Endpoint{
		service: s,
		app:     app,
		ep:      ep,
		route:   route,
		rules:   rules,
	}
	if snapshot != nil && len(snapshot.TrustedClients) > 0 {
		result.trusted = snapshot
	}

	return result
}

type taken struct {
	key    uint64
	window int64
}

// Check counts the request against every rule of the endpoint. It passes only
// when every rule allows it; a rejected request is counted by none of them,
// and retryAfter is the longest wait of the rules that rejected it. A trusted
// client of the snapshot is not counted at all.
func (e *Endpoint) Check(req Request) (bool, time.Duration) {
	client := newClientAddr(req.Client)
	if e.trusted != nil {
		if _, ok := e.trusted.TrustedPrefix(client.strictAddr(e.service.clientIP, req)); ok {
			return true, 0
		}
	}
	takenBuf := [4]taken{}
	takenList := takenBuf[:0]
	var rejectedBy *rule
	var retryAfter time.Duration

	for _, r := range e.rules {
		key := e.counterKey(r, req, &client)
		ok, window, wait := e.service.store.take(key, r.Requests, r.ParsedPeriod, rejectedBy == nil)
		if ok {
			if rejectedBy == nil {
				takenList = append(takenList, taken{key: key, window: window})
			}
			continue
		}
		if rejectedBy == nil {
			rejectedBy = r
		}
		retryAfter = max(retryAfter, wait)
	}

	if rejectedBy == nil {
		return true, 0
	}

	for _, t := range takenList {
		e.service.store.undo(t.key, t.window)
	}
	e.reject(rejectedBy, retryAfter, req, &client)
	req.Client.Limited(rejectedBy.label)

	return false, retryAfter
}

func (e *Endpoint) reject(r *rule, retryAfter time.Duration, req Request, client *clientAddr) {
	if r.rejected != nil {
		r.rejected.Inc()
	}

	ok, suppressed := r.warn.allow()
	if !ok {
		return
	}
	slog.Warn(
		"rate limit: request rejected",
		"app_name", e.app.Name,
		"app_id", e.app.Id,
		"endpoint_id", e.ep.Id,
		"route", e.route,
		"rule", r.label,
		"scope", r.Scope,
		"key", r.Key,
		"client_ip", client.get(e.service.clientIP, req).String(),
		"retry_after", retryAfter.Round(time.Millisecond).String(),
		"suppressed", suppressed,
	)
}

func (e *Endpoint) counterKey(r *rule, req Request, client *clientAddr) uint64 {
	var h maphash.Hash
	h.SetSeed(e.service.store.seed)

	var ruleBytes [8]byte
	binary.LittleEndian.PutUint64(ruleBytes[:], r.hash)
	_, _ = h.Write(ruleBytes[:])

	if r.Key == constant.RateLimitKeyAll {
		return h.Sum64()
	}
	if r.Key == constant.RateLimitKeyToken {
		if tag, value := credential(req.Authorization); value != "" {
			_ = h.WriteByte(tag)
			_, _ = h.WriteString(value)
			return h.Sum64()
		}
	}

	_ = h.WriteByte(tagIP)
	addr := limitAddr(client.get(e.service.clientIP, req)).As16()
	_, _ = h.Write(addr[:])

	return h.Sum64()
}

// credential is what a "token" counter is kept for: the bearer token whatever
// its spelling ("Bearer x", "bearer  x", bare "x"), or for basic auth the user
// name alone, so that guessing passwords of one user adds up in one counter.
// "" (an absent or unparseable value) counts by ip instead.
//
// Every new bearer token still gets a fresh counter by design, so a client
// sending random tokens is not held back by a token rule: pair it with an ip
// rule.
func credential(authorization string) (byte, string) {
	if user, _, ok := authModel.ParseBasic(authorization); ok {
		return tagBasicUser, user
	}
	return tagToken, authModel.ParseBearer(authorization)
}

// clientAddr resolves the client address once per request, and only when a
// rule needs it.
type clientAddr struct {
	addr netip.Addr
	// strict is the client address alone, without falling back to the peer:
	// trust is given only to an address the client really has.
	strict   netip.Addr
	resolved bool
}

func newClientAddr(client *clientstat.Client) clientAddr {
	if client == nil {
		return clientAddr{}
	}
	result := clientAddr{strict: client.Addr(), addr: client.Addr(), resolved: true}
	if !result.addr.IsValid() {
		result.addr = client.Peer()
	}
	return result
}

func (c *clientAddr) get(resolver *clientip.Resolver, req Request) netip.Addr {
	if c.resolved {
		return c.addr
	}
	c.resolved = true

	authReq := authModel.NewAuthRequest()
	authReq.SetHttpHeader(req.Header)
	authReq.SetRemoteAddr(req.RemoteAddr)
	c.strict = resolver.ClientIP(authReq)
	c.addr = c.strict
	// Every forwarded address is a trusted proxy: counting by the peer still
	// limits it, while a shared "unknown" counter would be one for everyone.
	if !c.addr.IsValid() {
		c.addr = authReq.ExtractRemoteAddrIP()
	}

	return c.addr
}

func (c *clientAddr) strictAddr(resolver *clientip.Resolver, req Request) netip.Addr {
	c.get(resolver, req)
	return c.strict
}

// limitAddr is the address a counter is kept for: an IPv6 client gets a whole
// /64, the smallest network a provider hands out, so rotating addresses inside
// it does not escape the limit.
func limitAddr(addr netip.Addr) netip.Addr {
	addr = addr.Unmap()
	if !addr.Is6() {
		return addr
	}
	prefix, err := addr.WithZone("").Prefix(64)
	if err != nil {
		return addr
	}
	return prefix.Addr()
}

// ruleIdentity tells counters apart: a rule keeps its counters while it stays
// the same, and starts from zero once any of its fields is edited.
func ruleIdentity(r *rateLimitModel.Rule) string {
	return strconv.Quote(r.Scope) + strconv.Quote(r.Name) + strconv.Quote(r.Key) +
		strconv.Itoa(r.Requests) + "/" + strconv.FormatInt(int64(r.ParsedPeriod), 10)
}

func ruleLabel(r *rateLimitModel.Rule) string {
	if r.Name != "" {
		return r.Name
	}
	return r.Key + " " + strconv.Itoa(r.Requests) + "/" + r.Period
}

// RetryAfterSeconds is the Retry-After value for a wait: whole seconds,
// rounded up, at least 1.
func RetryAfterSeconds(wait time.Duration) int {
	return max(1, int((wait+time.Second-1)/time.Second))
}
