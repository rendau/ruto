package mcp

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/rendau/ruto/internal/infra/clientip"
	"github.com/rendau/ruto/internal/infra/cloudflare"
)

func TestAccessLogClientIP(t *testing.T) {
	h := &Handler{clientIP: clientip.New([]string{"10.0.0.0/8"}, cloudflare.New("", 0))}

	var got string
	handler := h.withAccessLog(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got = clientIP(r)
	}))

	tests := []struct {
		name       string
		remoteAddr string
		headers    http.Header
		want       string
	}{
		{
			name:       "through cloudflare and traefik",
			remoteAddr: "10.0.0.10:443",
			headers:    http.Header{"X-Forwarded-For": {"172.68.10.5"}, "Cf-Connecting-Ip": {"95.56.1.2"}},
			want:       "95.56.1.2",
		},
		{
			name:       "around cloudflare with a forged header",
			remoteAddr: "10.0.0.10:443",
			headers:    http.Header{"X-Forwarded-For": {"203.0.113.10"}, "Cf-Connecting-Ip": {"8.8.8.8"}},
			want:       "203.0.113.10",
		},
		{
			name:       "forged left of x-forwarded-for",
			remoteAddr: "10.0.0.10:443",
			headers:    http.Header{"X-Forwarded-For": {"8.8.8.8, 203.0.113.10"}},
			want:       "203.0.113.10",
		},
		{
			name:       "untrusted peer",
			remoteAddr: "203.0.113.10:443",
			headers:    http.Header{"X-Forwarded-For": {"8.8.8.8"}, "Cf-Connecting-Ip": {"8.8.8.8"}},
			want:       "203.0.113.10",
		},
		{
			name:       "only trusted proxies",
			remoteAddr: "10.0.0.10:443",
			want:       "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got = "unset"
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tt.remoteAddr
			for k, vs := range tt.headers {
				req.Header[k] = vs
			}
			handler.ServeHTTP(httptest.NewRecorder(), req)
			assert.Equal(t, tt.want, got)
		})
	}
}
