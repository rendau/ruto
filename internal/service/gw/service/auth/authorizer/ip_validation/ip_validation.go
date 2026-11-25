package ip_validation

import (
	"log/slog"
	"net/netip"

	"github.com/samber/lo"

	authModel "github.com/rendau/ruto/internal/domain/auth/model"
	"github.com/rendau/ruto/internal/service/gw/service/auth/model"
	"github.com/rendau/ruto/internal/service/gw/service/clientip"
)

type Authorizer struct {
	allowedIPMap map[netip.Addr]struct{}
	clientIP     *clientip.Resolver
}

func New(conf *authModel.AuthMethodIPValidation, clientIP *clientip.Resolver) *Authorizer {
	allowedIPMap := lo.FilterSliceToMap(conf.AllowedIps, func(item authModel.AuthMethodIPValidationItem) (netip.Addr, struct{}, bool) {
		addr, err := netip.ParseAddr(item.Ip)
		if err != nil {
			slog.Warn("ip_validation: skip invalid allowed ip: " + item.Ip)
			return netip.Addr{}, struct{}{}, false
		}
		return addr, struct{}{}, true
	})

	return &Authorizer{
		allowedIPMap: allowedIPMap,
		clientIP:     clientIP,
	}
}

func (a *Authorizer) Authorize(req *model.AuthRequest) model.Result {
	clientIP := a.clientIP.ClientIP(req)
	if a.checkIP(clientIP) {
		return model.Allowed
	}

	slog.Warn(
		"ip_validation: access denied",
		"forwarded_for", lo.Map(req.ExtractForwardedFor(), func(ip netip.Addr, _ int) string { return ip.String() }),
		"real_ip", req.ExtractRealIP().String(),
		"cf_connecting_ip", req.ExtractCfConnectingIP().String(),
		"client_ip", clientIP.String(),
		"allowed_ips", lo.Map(lo.Keys(a.allowedIPMap), func(ip netip.Addr, _ int) string { return ip.String() }),
		"trusted_proxy_addresses", lo.Map(a.clientIP.Trusted(), func(ip netip.Prefix, _ int) string { return ip.String() }),
		"remote_addr", req.RemoteAddr,
	)

	return model.Denied
}

func (a *Authorizer) checkIP(ip netip.Addr) bool {
	if !ip.IsValid() {
		return false
	}
	_, ok := a.allowedIPMap[ip]
	return ok
}
