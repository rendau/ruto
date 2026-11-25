package clientstat

import (
	"context"
	"net/http"
	"net/netip"
	"time"

	rootModel "github.com/rendau/ruto/internal/domain/root/model"
	authModel "github.com/rendau/ruto/internal/service/gw/service/auth/model"
	"github.com/rendau/ruto/internal/service/gw/service/clientip"
)

// Service is made once per gateway and shared by the handlers of every
// snapshot, like the collector it writes to.
type Service struct {
	collector *Collector
	clientIP  *clientip.Resolver
}

// New counts into collector; clientIP must be the resolver of the block list
// and rate limits, so that all three see the same client.
func New(collector *Collector, clientIP *clientip.Resolver) *Service {
	return &Service{collector: collector, clientIP: clientIP}
}

// Meter counts the requests of one snapshot's handlers.
type Meter struct {
	service *Service
	// the snapshot, when it has trusted clients
	trusted *rootModel.Root
}

// Meter builds the counter for the handlers of snapshot. A nil Service gives
// a nil Meter, which counts nothing.
func (s *Service) Meter(snapshot *rootModel.Root) *Meter {
	if s == nil {
		return nil
	}
	m := &Meter{service: s}
	if snapshot != nil && len(snapshot.TrustedClients) > 0 {
		m.trusted = snapshot
	}
	return m
}

// Client is the client of one request, resolved once by Start; the block list
// and rate limits read its address and mark what they refused.
type Client struct {
	collector *Collector
	key       key
	addr      netip.Addr
	peer      netip.Addr
}

// Start counts a request and resolves its client the way the block list and
// rate limits do (clientip, by TRUSTED_PROXY_ADDRESSES). A trusted client of
// the snapshot is counted under its trusted entry, so trusted traffic takes
// one row per entry whatever the number of addresses behind it.
func (m *Meter) Start(remoteAddr string, header http.Header) *Client {
	if m == nil {
		return nil
	}

	var req authModel.AuthRequest
	req.SetHttpHeader(header)
	req.SetRemoteAddr(remoteAddr)

	c := &Client{
		collector: m.service.collector,
		addr:      m.service.clientIP.ClientIP(&req),
		peer:      req.ExtractRemoteAddrIP(),
	}
	c.key = clientKey(c.addr)
	if m.trusted != nil && c.addr.IsValid() {
		if tc, ok := m.trusted.TrustedPrefix(c.addr); ok {
			c.key = trustedKey(tc)
		}
	}

	c.collector.record(c.key, counter{requests: 1})
	return c
}

// Addr is the client address alone, invalid when it cannot be told: trust and
// blocks are given only to an address the client really has.
func (c *Client) Addr() netip.Addr {
	if c == nil {
		return netip.Addr{}
	}
	return c.addr
}

// Peer is the address of the connection, the client or the last proxy.
func (c *Client) Peer() netip.Addr {
	if c == nil {
		return netip.Addr{}
	}
	return c.peer
}

// Blocked counts the request as refused by the block list.
func (c *Client) Blocked() {
	if c == nil {
		return
	}
	c.collector.record(c.key, counter{blocked: 1})
}

// Limited counts the request as rejected by the rate-limit rule labelled rule.
func (c *Client) Limited(rule string) {
	if c == nil {
		return
	}
	c.collector.record(c.key, counter{limited: 1, lastRule: rule, ruleAt: time.Now().UnixNano()})
}

// trustedKey names the trusted entry; an entry without a name is named by its
// network, so its row is still told apart from untrusted ones.
func trustedKey(tc *rootModel.RootTrustedClient) key {
	name := tc.Name
	if name == "" {
		name = tc.Prefix.String()
	}
	return key{prefix: tc.Prefix, trusted: name}
}

type contextKey struct{}

func WithClient(ctx context.Context, c *Client) context.Context {
	if c == nil {
		return ctx
	}
	return context.WithValue(ctx, contextKey{}, c)
}

// FromContext is the client Start resolved for the request, nil when the
// request was not counted.
func FromContext(ctx context.Context) *Client {
	c, _ := ctx.Value(contextKey{}).(*Client)
	return c
}
