// Package blocklist refuses the clients core has blocked (BlockedClient), on
// every route of the gateway.
//
// The list does not come with the snapshot: core_client polls
// BlockedClient.GetVersion and reloads the list when core pushes
// "check_blocklist", so a block takes effect within seconds and without a
// publish. Service holds the list for the whole life of the gateway and swaps
// it atomically; the handlers of every snapshot read it through a Guard, which
// adds the trusted clients of that snapshot: a trusted client is never
// blocked.
package blocklist

import (
	"log/slog"
	"net/http"
	"net/netip"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/samber/lo"

	rootModel "github.com/rendau/ruto/internal/domain/root/model"
	authModel "github.com/rendau/ruto/internal/service/gw/service/auth/model"
	"github.com/rendau/ruto/internal/service/gw/service/clientip"
	"github.com/rendau/ruto/internal/service/gw/service/clientstat"
	"github.com/rendau/ruto/internal/service/gw/service/metrics"
)

const (
	ProtocolHTTP = "http"
	ProtocolGRPC = "grpc"

	blockWarnEvery = 10 * time.Second
	invalidSamples = 5
)

type Service struct {
	clientIP *clientip.Resolver
	current  atomic.Pointer[table]
	warn     throttle
	now      func() time.Time
}

// New gives an empty list: until the first successful fetch nobody is
// blocked.
func New(clientIP *clientip.Resolver) *Service {
	return &Service{
		clientIP: clientIP,
		warn:     throttle{every: blockWarnEvery},
		now:      time.Now,
	}
}

// Set replaces the list. Rules that are not an address or a network are
// skipped, with one warning per applied version.
func (s *Service) Set(version string, rules []Rule) {
	t, invalid := buildTable(version, rules, s.now().Unix())
	s.current.Store(t)
	metrics.SetBlocklistRules(t.size)

	if len(invalid) > 0 {
		slog.Warn(
			"blocklist: rules skipped, not an address or network",
			"version", version,
			"count", len(invalid),
			"values", lo.Subset(invalid, 0, invalidSamples),
		)
	}
}

// Version is the version of the applied list, "" before the first one.
func (s *Service) Version() string {
	if t := s.current.Load(); t != nil {
		return t.version
	}
	return ""
}

// Applied reports whether a list was ever set.
func (s *Service) Applied() bool {
	return s.current.Load() != nil
}

// Len is the number of active networks of the applied list, as of its fetch.
func (s *Service) Len() int {
	if t := s.current.Load(); t != nil {
		return t.size
	}
	return 0
}

// Guard is the check of one snapshot's handlers for one protocol.
type Guard struct {
	service  *Service
	trusted  *rootModel.Root
	protocol string
	blocked  prometheus.Counter
}

// Guard builds the check for the handlers of snapshot. A nil Service gives a
// nil Guard, which blocks nobody.
func (s *Service) Guard(snapshot *rootModel.Root, protocol string) *Guard {
	if s == nil {
		return nil
	}
	g := &Guard{
		service:  s,
		protocol: protocol,
		blocked:  metrics.Blocked(protocol),
	}
	if snapshot != nil && len(snapshot.TrustedClients) > 0 {
		g.trusted = snapshot
	}
	return g
}

// Active reports whether there is anything to check: callers may skip
// building the request data otherwise.
func (g *Guard) Active() bool {
	if g == nil {
		return false
	}
	t := g.service.current.Load()
	return t != nil && t.size > 0
}

// Blocked reports whether the client of a request is blocked. The client is
// found the way rate limits and ip_validation find it (clientip, by
// TRUSTED_PROXY_ADDRESSES); a client whose address cannot be told is not
// blocked, nor is a trusted client of the snapshot.
func (g *Guard) Blocked(remoteAddr string, header http.Header) bool {
	if g == nil {
		return false
	}
	t := g.service.current.Load()
	if t == nil || t.size == 0 {
		return false
	}

	var req authModel.AuthRequest
	req.SetHttpHeader(header)
	req.SetRemoteAddr(remoteAddr)
	return g.check(t, g.service.clientIP.ClientIP(&req))
}

// BlockedClient is Blocked for a request whose client clientstat already
// resolved; a refusal is counted in the client's stats. Without a client it
// resolves the address itself.
func (g *Guard) BlockedClient(client *clientstat.Client, remoteAddr string, header http.Header) bool {
	if client == nil {
		return g.Blocked(remoteAddr, header)
	}
	if g == nil {
		return false
	}
	t := g.service.current.Load()
	if t == nil || t.size == 0 {
		return false
	}
	if !g.check(t, client.Addr()) {
		return false
	}
	client.Blocked()
	return true
}

func (g *Guard) check(t *table, ip netip.Addr) bool {
	if !ip.IsValid() {
		return false
	}

	rule, ok := t.lookup(ip, g.service.now().Unix())
	if !ok {
		return false
	}
	if g.trusted != nil {
		if _, isTrusted := g.trusted.TrustedPrefix(ip); isTrusted {
			return false
		}
	}

	g.reject(ip, rule)
	return true
}

func (g *Guard) reject(ip netip.Addr, rule netip.Prefix) {
	if g.blocked != nil {
		g.blocked.Inc()
	}

	ok, suppressed := g.service.warn.allow()
	if !ok {
		return
	}
	slog.Warn(
		"blocklist: request refused",
		"client_ip", ip.Unmap().String(),
		"rule", rule.String(),
		"protocol", g.protocol,
		"suppressed", suppressed,
	)
}
