// Package clientip finds the address of the client behind the proxies in front
// of the gateway. Forwarding headers are trusted only when the request came
// from TRUSTED_PROXY_ADDRESSES: anyone else could put any address there.
package clientip

import (
	"log/slog"
	"net/netip"
	"strings"

	"github.com/rendau/ruto/internal/service/gw/service/auth/model"
)

type Resolver struct {
	trusted []netip.Prefix
}

func New(trustedProxyAddresses []string) *Resolver {
	return &Resolver{trusted: parseTrusted(trustedProxyAddresses)}
}

// ClientIP is the peer address when the peer is not a trusted proxy. Behind
// one it is the rightmost X-Forwarded-For address that is not a trusted proxy:
// every proxy appends the address it got the request from, so the addresses
// to the left of it were written by the client itself. Only when
// X-Forwarded-For names nobody but trusted proxies does X-Real-Ip count
// (traefik overwrites it). Other client-address headers (Cf-Connecting-Ip,
// True-Client-Ip and the like) are never read: a proxy that does not know them
// passes them on as the client sent them.
//
// The result is invalid when the peer address is unknown, the chain holds an
// entry that is not an address, or every address is a trusted proxy.
func (r *Resolver) ClientIP(req *model.AuthRequest) netip.Addr {
	remoteIP := req.ExtractRemoteAddrIP()
	if !remoteIP.IsValid() {
		return netip.Addr{}
	}
	if !r.IsTrusted(remoteIP) {
		return remoteIP
	}

	ips := req.ExtractForwardedFor()
	for i := len(ips) - 1; i >= 0; i-- {
		if !ips[i].IsValid() {
			return netip.Addr{}
		}
		if !r.IsTrusted(ips[i]) {
			return ips[i]
		}
	}

	if realIP := req.ExtractRealIP(); realIP.IsValid() && !r.IsTrusted(realIP) {
		return realIP
	}

	return netip.Addr{}
}

func (r *Resolver) IsTrusted(ip netip.Addr) bool {
	for _, subnet := range r.trusted {
		if subnet.Contains(ip) {
			return true
		}
	}
	return false
}

func (r *Resolver) Trusted() []netip.Prefix {
	return r.trusted
}

func parseTrusted(items []string) []netip.Prefix {
	result := make([]netip.Prefix, 0, len(items))
	for _, raw := range items {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}

		if prefix, err := netip.ParsePrefix(raw); err == nil {
			result = append(result, prefix.Masked())
			continue
		}

		if ip, err := netip.ParseAddr(raw); err == nil {
			result = append(result, netip.PrefixFrom(ip, ip.BitLen()))
			continue
		}

		slog.Warn("clientip: skip invalid trusted proxy address", "value", raw)
	}
	return result
}
