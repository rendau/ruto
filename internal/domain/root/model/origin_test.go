package model

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeOrigin(t *testing.T) {
	tests := []struct {
		raw     string
		want    string
		wantErr string
	}{
		{raw: "*", want: "*"},
		{raw: " https://App.Example.com/ ", want: "https://app.example.com"},
		{raw: "http://localhost:5173", want: "http://localhost:5173"},
		{raw: "HTTPS://*.Example.com:8443", want: "https://*.example.com:8443"},
		{raw: "app.example.com", wantErr: "scheme must be http or https"},
		{raw: "ftp://app.example.com", wantErr: "scheme must be http or https"},
		{raw: "https://app.example.com/api", wantErr: "must be an origin"},
		{raw: "https://app.example.com?x=1", wantErr: "must be an origin"},
		{raw: "https://*", wantErr: "wildcard"},
		{raw: "https://*.*.example.com", wantErr: "wildcard"},
		{raw: "https://app.*.example.com", wantErr: "wildcard"},
		{raw: "https://*app.example.com", wantErr: "wildcard"},
	}
	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			got, err := normalizeOrigin(tt.raw)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRootCorsNormalizeOrigins(t *testing.T) {
	cors := RootCors{AllowOrigins: []string{" ", "https://A.example.com/", "https://a.example.com", "*"}}
	require.NoError(t, cors.Normalize())
	assert.Equal(t, []string{"https://a.example.com", "*"}, cors.AllowOrigins)

	cors = RootCors{AllowOrigins: []string{"https://ok.example.com", "nope"}}
	require.ErrorContains(t, cors.Normalize(), "allow_origins[1]: scheme must be http or https")
}

func TestOriginMatcher(t *testing.T) {
	m := NewOriginMatcher([]string{"https://app.example.com", "https://*.fronts.example", "http://*.localhost:5173"})
	assert.False(t, m.Any())

	for _, origin := range []string{
		"https://app.example.com",
		"https://APP.example.com",
		"https://shop.fronts.example",
		"https://a.b.fronts.example",
		"http://admin.localhost:5173",
	} {
		assert.True(t, m.Listed(origin), origin)
	}
	for _, origin := range []string{
		"http://app.example.com",       // scheme differs
		"https://app.example.com:8443", // port differs
		"https://fronts.example",       // the apex itself is not a subdomain
		"https://evilfronts.example",   // suffix without the dot
		"https://shop.fronts.example.evil.example",
		"http://admin.localhost",    // pattern names a port
		"http://admin.localhost:80", // other port
		"",
	} {
		assert.False(t, m.Listed(origin), origin)
	}

	any := NewOriginMatcher([]string{"*", "https://app.example.com"})
	assert.True(t, any.Any())
	assert.True(t, any.Listed("https://app.example.com"))
	assert.False(t, any.Listed("https://other.example.com"), "* is not a listed origin")
}

func TestRootCorsMaxAge(t *testing.T) {
	for raw, want := range map[string]string{"": "", " 600 ": "600", "24h": "86400", "90m": "5400"} {
		cors := RootCors{MaxAge: raw}
		require.NoError(t, cors.Normalize(), raw)
		assert.Equal(t, strings.TrimSpace(raw), cors.MaxAge, "kept as written")
		assert.Equal(t, want, cors.MaxAgeSeconds(), raw)
	}
	for _, raw := range []string{"soon", "-5", "-1h"} {
		cors := RootCors{MaxAge: raw}
		require.ErrorContains(t, cors.Normalize(), "max_age", raw)
		assert.Empty(t, cors.MaxAgeSeconds(), raw)
	}
}
