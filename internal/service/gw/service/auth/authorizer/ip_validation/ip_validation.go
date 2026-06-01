package ip_validation

import (
	"log/slog"
	"slices"

	"github.com/samber/lo"

	authModel "github.com/rendau/ruto/internal/domain/auth/model"
	"github.com/rendau/ruto/internal/service/gw/service/auth/model"
)

type Authorizer struct {
	allowedIPMap map[string]bool
}

func New(conf *authModel.AuthMethodIPValidation) *Authorizer {
	return &Authorizer{
		allowedIPMap: lo.SliceToMap(conf.AllowedIps, func(ip string) (string, bool) { return ip, true }),
	}
}

func (a *Authorizer) Authorize(req *model.AuthRequest) bool {
	// if len(a.allowedIPMap) == 0 {
	// 	return false
	// }
	// for _, ip := range req.Ips {
	// 	if a.checkIP(ip) {
	// 		return true
	// 	}
	// }
	// return false

	// todo: temporary disable
	allowedIPs := lo.Keys(a.allowedIPMap)
	slices.Sort(allowedIPs)

	slog.Info(
		"ip_validation is temporarily disabled: allow all",
		"ips", req.ExtractIPs(),
		"allowed_ips", allowedIPs,
		"remote_addr", req.RemoteAddr,
	)
	return true
}

func (a *Authorizer) checkIP(ip string) bool {
	if ip == "" {
		return false
	}
	return a.allowedIPMap[ip]
}
