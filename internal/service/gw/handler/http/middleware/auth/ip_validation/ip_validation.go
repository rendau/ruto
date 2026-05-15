package ip_validation

import (
	"net"
	"net/http"

	"github.com/samber/lo"

	authModel "github.com/rendau/ruto/internal/domain/auth/model"
)

type Authorizer struct {
	allowedIPMap map[string]bool
}

func New(conf *authModel.AuthMethodIPValidation) *Authorizer {
	return &Authorizer{
		allowedIPMap: lo.SliceToMap(
			conf.AllowedIps,
			func(ip string) (string, bool) {
				return ip, true
			},
		),
	}
}

func (a Authorizer) Authorize(r *http.Request) bool {
	if len(a.allowedIPMap) == 0 {
		return true
	}
	return a.checkIP(extractRemoteIP(r.RemoteAddr))
}

func extractRemoteIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return remoteAddr
	}
	return host
}

func (a Authorizer) checkIP(ip string) bool {
	return a.allowedIPMap[ip]
}
