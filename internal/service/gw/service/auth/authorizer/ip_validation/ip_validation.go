package ip_validation

import (
	"log/slog"
	"net/netip"
	"strings"

	"github.com/samber/lo"

	authModel "github.com/rendau/ruto/internal/domain/auth/model"
	"github.com/rendau/ruto/internal/service/gw/service/auth/model"
)

type Authorizer struct {
	allowedIPMap   map[netip.Addr]struct{}
	trustedSubnets []netip.Prefix
}

func New(conf *authModel.AuthMethodIPValidation, trustedProxyAddresses []string) *Authorizer {
	allowedIPMap := lo.FilterSliceToMap(conf.AllowedIps, func(ip string) (netip.Addr, struct{}, bool) {
		addr, err := netip.ParseAddr(strings.TrimSpace(ip))
		if err != nil {
			slog.Warn("ip_validation: skip invalid allowed ip: " + ip)
			return netip.Addr{}, struct{}{}, false
		}
		return addr, struct{}{}, true
	})

	return &Authorizer{
		allowedIPMap:   allowedIPMap,
		trustedSubnets: parseTrustedSubnets(trustedProxyAddresses),
	}
}

func (a *Authorizer) Authorize(req *model.AuthRequest) bool {
	clientIP := a.extractClientIP(req)
	if a.checkIP(clientIP) {
		return true
	}

	slog.Info(
		"ip_validation: access denied",
		"ips", lo.Map(req.ExtractIPAddrs(), func(ip netip.Addr, _ int) string { return ip.String() }),
		"client_ip", clientIP.String(),
		"allowed_ips", lo.Map(lo.Keys(a.allowedIPMap), func(ip netip.Addr, _ int) string { return ip.String() }),
		"trusted_proxy_addresses", lo.Map(a.trustedSubnets, func(ip netip.Prefix, _ int) string { return ip.String() }),
		"remote_addr", req.RemoteAddr,
	)

	return false
}

func (a *Authorizer) checkIP(ip netip.Addr) bool {
	if !ip.IsValid() {
		return false
	}
	_, ok := a.allowedIPMap[ip]
	return ok
}

func (a *Authorizer) extractClientIP(req *model.AuthRequest) netip.Addr {
	remoteIP := req.ExtractRemoteAddrIP()
	if !remoteIP.IsValid() {
		return netip.Addr{}
	}
	if !a.isTrustedProxy(remoteIP) {
		return remoteIP
	}

	ips := req.ExtractIPAddrs()
	for i := len(ips) - 1; i >= 0; i-- {
		if !a.isTrustedProxy(ips[i]) {
			return ips[i]
		}
	}

	return netip.Addr{}
}

func (a *Authorizer) isTrustedProxy(ip netip.Addr) bool {
	for _, subnet := range a.trustedSubnets {
		if subnet.Contains(ip) {
			return true
		}
	}

	return false
}

func parseTrustedSubnets(items []string) []netip.Prefix {
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
			bits := 32
			if ip.Is6() {
				bits = 128
			}
			result = append(result, netip.PrefixFrom(ip, bits))
			continue
		}

		slog.Warn("ip_validation: skip invalid trusted proxy address", "value", raw)
	}
	return result
}
