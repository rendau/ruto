package clientip

import (
	"net/http"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolver_ClientIP(t *testing.T) {
	resolver := New([]string{"10.0.0.0/8", "192.0.2.1", "bad"}, nil)

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
			req := HTTPRequest(&http.Request{Header: headers, RemoteAddr: tt.remoteAddr})

			assert.Equal(t, tt.want, resolver.ClientIP(req).String())
		})
	}

	assert.Len(t, resolver.Trusted(), 2)
}

func TestResolver_IgnoresClientHeaders(t *testing.T) {
	resolver := New([]string{"10.0.0.0/8"}, nil)

	for _, header := range []string{"Cf-Connecting-Ip", "True-Client-Ip", "X-Client-Ip", "X-Cluster-Client-Ip"} {
		t.Run(header, func(t *testing.T) {
			req := HTTPRequest(&http.Request{Header: http.Header{"X-Forwarded-For": {"203.0.113.10"}, header: {"1.1.1.1"}}, RemoteAddr: "10.0.0.10:443"})
			assert.Equal(t, "203.0.113.10", resolver.ClientIP(req).String())

			req = HTTPRequest(&http.Request{Header: http.Header{header: {"1.1.1.1"}}, RemoteAddr: "10.0.0.10:443"})
			assert.False(t, resolver.ClientIP(req).IsValid())
		})
	}
}

type cloudflareStub []netip.Prefix

func (c cloudflareStub) Contains(ip netip.Addr) bool {
	for _, network := range c {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

func TestResolver_Cloudflare(t *testing.T) {
	resolver := New([]string{"10.0.0.0/8"}, cloudflareStub{netip.MustParsePrefix("172.64.0.0/13")})

	tests := []struct {
		name       string
		remoteAddr string
		headers    http.Header
		want       string
	}{
		{
			name:       "traefik dropped the chain from cloudflare",
			remoteAddr: "10.0.0.10:443",
			headers:    http.Header{"X-Forwarded-For": {"172.68.10.5"}, "Cf-Connecting-Ip": {"95.56.1.2"}},
			want:       "95.56.1.2",
		},
		{
			name:       "traefik kept the chain from cloudflare",
			remoteAddr: "10.0.0.10:443",
			headers:    http.Header{"X-Forwarded-For": {"1.1.1.1, 95.56.1.2, 172.68.10.5"}, "Cf-Connecting-Ip": {"95.56.1.2"}},
			want:       "95.56.1.2",
		},
		{
			name:       "cloudflare is the peer",
			remoteAddr: "172.68.10.5:443",
			headers:    http.Header{"Cf-Connecting-Ip": {"2001:db8::7"}},
			want:       "2001:db8::7",
		},
		{
			name:       "cloudflare in x-real-ip",
			remoteAddr: "10.0.0.10:443",
			headers:    http.Header{"X-Real-Ip": {"172.68.10.5"}, "Cf-Connecting-Ip": {"95.56.1.2"}},
			want:       "95.56.1.2",
		},
		{
			name:       "cloudflare without the header",
			remoteAddr: "10.0.0.10:443",
			headers:    http.Header{"X-Forwarded-For": {"172.68.10.5"}},
			want:       "172.68.10.5",
		},
		{
			name:       "cloudflare with garbage in the header",
			remoteAddr: "10.0.0.10:443",
			headers:    http.Header{"X-Forwarded-For": {"172.68.10.5"}, "Cf-Connecting-Ip": {"garbage"}},
			want:       "172.68.10.5",
		},
		{
			name:       "direct client forges the header",
			remoteAddr: "10.0.0.10:443",
			headers:    http.Header{"X-Forwarded-For": {"203.0.113.10"}, "Cf-Connecting-Ip": {"95.56.1.2"}},
			want:       "203.0.113.10",
		},
		{
			name:       "client forges cloudflare in the chain",
			remoteAddr: "10.0.0.10:443",
			headers:    http.Header{"X-Forwarded-For": {"172.68.10.5, 203.0.113.10"}, "Cf-Connecting-Ip": {"95.56.1.2"}},
			want:       "203.0.113.10",
		},
		{
			name:       "untrusted peer forges the header",
			remoteAddr: "203.0.113.10:443",
			headers:    http.Header{"Cf-Connecting-Ip": {"95.56.1.2"}},
			want:       "203.0.113.10",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := HTTPRequest(&http.Request{Header: tt.headers, RemoteAddr: tt.remoteAddr})
			assert.Equal(t, tt.want, resolver.ClientIP(req).String())
		})
	}
}

func TestParse(t *testing.T) {
	assert.Equal(t, "198.51.100.1", ParseAddr(" 198.51.100.1:443 ").String())
	assert.Equal(t, "2001:db8::1", ParseAddr("[2001:db8::1]:443").String())
	assert.Equal(t, "2001:db8::1", ParseAddr("2001:db8::1").String())
	assert.False(t, ParseAddr("garbage").IsValid())
	assert.False(t, ParseAddr("").IsValid())

	ips := ParseForwardedFor([]string{"198.51.100.1, garbage", " , 192.0.2.10"})
	require.Len(t, ips, 3)
	assert.Equal(t, "198.51.100.1", ips[0].String())
	assert.False(t, ips[1].IsValid())
	assert.Equal(t, "192.0.2.10", ips[2].String())
}
