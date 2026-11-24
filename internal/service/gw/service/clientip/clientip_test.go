package clientip

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/rendau/ruto/internal/service/gw/service/auth/model"
)

func TestResolver_ClientIP(t *testing.T) {
	resolver := New([]string{"10.0.0.0/8", "192.0.2.1", "bad"})

	tests := []struct {
		name       string
		remoteAddr string
		xff        string
		headers    http.Header
		want       string
	}{
		{name: "untrusted peer, header ignored", remoteAddr: "203.0.113.10:443", xff: "1.1.1.1", want: "203.0.113.10"},
		{name: "trusted peer, rightmost untrusted", remoteAddr: "10.0.0.10:443", xff: "1.1.1.1, 198.51.100.1, 10.0.0.3", want: "198.51.100.1"},
		{name: "trusted single address", remoteAddr: "192.0.2.1:443", xff: "198.51.100.1", want: "198.51.100.1"},
		{name: "all trusted", remoteAddr: "10.0.0.10:443", xff: "10.0.0.2", want: "invalid IP"},
		{name: "no peer", remoteAddr: "", xff: "1.1.1.1", want: "invalid IP"},
		{name: "broken chain", remoteAddr: "10.0.0.10:443", xff: "1.1.1.1, garbage", want: "invalid IP"},
		{name: "real ip without xff", remoteAddr: "10.0.0.10:443", headers: http.Header{"X-Real-Ip": {"198.51.100.7"}}, want: "198.51.100.7"},
		{name: "real ip when xff is all trusted", remoteAddr: "10.0.0.10:443", xff: "10.0.0.2", headers: http.Header{"X-Real-Ip": {"198.51.100.7"}}, want: "198.51.100.7"},
		{name: "real ip loses to xff", remoteAddr: "10.0.0.10:443", xff: "198.51.100.1", headers: http.Header{"X-Real-Ip": {"198.51.100.7"}}, want: "198.51.100.1"},
		{name: "untrusted peer ignores real ip", remoteAddr: "203.0.113.10:443", headers: http.Header{"X-Real-Ip": {"198.51.100.7"}}, want: "203.0.113.10"},
		{name: "xff over several lines", remoteAddr: "10.0.0.10:443", headers: http.Header{"X-Forwarded-For": {"198.51.100.9", "198.51.100.1"}}, want: "198.51.100.1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			headers := http.Header{}
			if tt.xff != "" {
				headers.Set("X-Forwarded-For", tt.xff)
			}
			for k, vs := range tt.headers {
				headers[k] = vs
			}
			req := model.NewAuthRequest()
			req.SetHttpHeader(headers)
			req.SetRemoteAddr(tt.remoteAddr)

			assert.Equal(t, tt.want, resolver.ClientIP(req).String())
		})
	}

	assert.Len(t, resolver.Trusted(), 2)
}

func TestResolver_IgnoresClientHeaders(t *testing.T) {
	resolver := New([]string{"10.0.0.0/8"})

	for _, header := range []string{"Cf-Connecting-Ip", "True-Client-Ip", "X-Client-Ip", "X-Cluster-Client-Ip"} {
		t.Run(header, func(t *testing.T) {
			req := model.NewAuthRequest()
			req.SetHttpHeader(http.Header{"X-Forwarded-For": {"203.0.113.10"}, header: {"1.1.1.1"}})
			req.SetRemoteAddr("10.0.0.10:443")
			assert.Equal(t, "203.0.113.10", resolver.ClientIP(req).String())

			req = model.NewAuthRequest()
			req.SetHttpHeader(http.Header{header: {"1.1.1.1"}})
			req.SetRemoteAddr("10.0.0.10:443")
			assert.False(t, resolver.ClientIP(req).IsValid())
		})
	}
}
