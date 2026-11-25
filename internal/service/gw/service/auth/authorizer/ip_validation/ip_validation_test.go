package ip_validation

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	authModel "github.com/rendau/ruto/internal/domain/auth/model"
	authReqModel "github.com/rendau/ruto/internal/service/gw/service/auth/model"
	"github.com/rendau/ruto/internal/service/gw/service/clientip"
)

func TestAuthorizeTrustedProxyFlow(t *testing.T) {
	tests := []struct {
		name                  string
		allowedIPs            []string
		trustedProxyAddresses []string
		remoteAddr            string
		xff                   string
		want                  authReqModel.Result
	}{
		{
			name:                  "remote is not trusted so forwarded headers are ignored",
			allowedIPs:            []string{"1.1.1.1"},
			trustedProxyAddresses: []string{"10.0.0.0/8"},
			remoteAddr:            "203.0.113.10:443",
			xff:                   "1.1.1.1",
			want:                  authReqModel.Denied,
		},
		{
			name:                  "remote is trusted and client is detected from right to left",
			allowedIPs:            []string{"1.1.1.1"},
			trustedProxyAddresses: []string{"10.0.0.0/8"},
			remoteAddr:            "10.0.0.10:443",
			xff:                   "1.1.1.1, 10.0.0.2, 10.0.0.3",
			want:                  authReqModel.Allowed,
		},
		{
			name:                  "all addresses are trusted proxies",
			allowedIPs:            []string{"1.1.1.1"},
			trustedProxyAddresses: []string{"10.0.0.0/8"},
			remoteAddr:            "10.0.0.10:443",
			xff:                   "10.0.0.2, 10.0.0.3",
			want:                  authReqModel.Denied,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &authReqModel.AuthRequest{
				Headers: http.Header{
					"X-Forwarded-For": {tt.xff},
				},
			}
			req.SetRemoteAddr(tt.remoteAddr)

			allowedIps := make([]authModel.AuthMethodIPValidationItem, len(tt.allowedIPs))
			for i, ip := range tt.allowedIPs {
				allowedIps[i] = authModel.AuthMethodIPValidationItem{Ip: ip}
			}

			got := New(&authModel.AuthMethodIPValidation{
				AllowedIps: allowedIps,
			}, clientip.New(tt.trustedProxyAddresses, nil)).Authorize(req)

			require.Equal(t, tt.want, got)
		})
	}
}

func TestAuthorizeIgnoresSpoofableClientHeaders(t *testing.T) {
	authorizer := New(&authModel.AuthMethodIPValidation{
		AllowedIps: []authModel.AuthMethodIPValidationItem{{Ip: "1.1.1.1"}},
	}, clientip.New([]string{"10.0.0.0/8"}, nil))

	for _, header := range []string{"Cf-Connecting-Ip", "True-Client-Ip", "X-Client-Ip", "X-Cluster-Client-Ip"} {
		t.Run(header, func(t *testing.T) {
			// traefik appended the real client; the header is the client's own
			req := &authReqModel.AuthRequest{Headers: http.Header{
				"X-Forwarded-For": {"203.0.113.10"},
				header:            {"1.1.1.1"},
			}}
			req.SetRemoteAddr("10.0.0.10:443")
			require.Equal(t, authReqModel.Denied, authorizer.Authorize(req))

			// nor without X-Forwarded-For
			req = &authReqModel.AuthRequest{Headers: http.Header{header: {"1.1.1.1"}}}
			req.SetRemoteAddr("10.0.0.10:443")
			require.Equal(t, authReqModel.Denied, authorizer.Authorize(req))
		})
	}
}

func TestAuthorizeRealIPOnlyWithoutForwardedFor(t *testing.T) {
	authorizer := New(&authModel.AuthMethodIPValidation{
		AllowedIps: []authModel.AuthMethodIPValidationItem{{Ip: "1.1.1.1"}},
	}, clientip.New([]string{"10.0.0.0/8"}, nil))

	req := &authReqModel.AuthRequest{Headers: http.Header{"X-Real-Ip": {"1.1.1.1"}}}
	req.SetRemoteAddr("10.0.0.10:443")
	require.Equal(t, authReqModel.Allowed, authorizer.Authorize(req))

	req = &authReqModel.AuthRequest{Headers: http.Header{"X-Real-Ip": {"1.1.1.1"}, "X-Forwarded-For": {"203.0.113.10"}}}
	req.SetRemoteAddr("10.0.0.10:443")
	require.Equal(t, authReqModel.Denied, authorizer.Authorize(req))

	// an untrusted peer is never taken for another address
	req = &authReqModel.AuthRequest{Headers: http.Header{"X-Real-Ip": {"1.1.1.1"}}}
	req.SetRemoteAddr("203.0.113.10:443")
	require.Equal(t, authReqModel.Denied, authorizer.Authorize(req))
}
