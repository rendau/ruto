package model

import (
	"encoding/base64"
	"net/http"
	"net/netip"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAuthRequestExtractBasic(t *testing.T) {
	token := base64.StdEncoding.EncodeToString([]byte("admin:secret"))
	headers := http.Header{}
	headers.Set("Authorization", "Basic "+token)

	req := &AuthRequest{
		Headers: headers,
	}

	username, password := req.ExtractBasic()
	require.Equal(t, "admin", username)
	require.Equal(t, "secret", password)
}

func TestAuthRequestExtractTokenFromLowercaseHeader(t *testing.T) {
	req := &AuthRequest{
		Headers: http.Header{
			"authorization": {"Bearer token"},
		},
	}

	require.Equal(t, "token", req.ExtractToken())
}

func TestAuthRequestExtractAPIKey(t *testing.T) {
	headers := http.Header{}
	headers.Set("X-Api-Key", "from-header")

	req := &AuthRequest{
		Headers: headers,
		QueryParams: url.Values{
			"x-api-key": {"from-query"},
		},
	}

	require.Equal(t, "from-header", req.ExtractAPIKey("X-Api-Key"))
}

func TestAuthRequestExtractAPIKeyFromLowercaseHeader(t *testing.T) {
	req := &AuthRequest{
		Headers: http.Header{
			"x-api-key": {"from-header"},
		},
		QueryParams: url.Values{
			"x-api-key": {"from-query"},
		},
	}

	require.Equal(t, "from-header", req.ExtractAPIKey("X-Api-Key"))
}

func TestAuthRequestExtractForwardedFor(t *testing.T) {
	headers := http.Header{}
	headers.Add("X-Forwarded-For", "10.0.0.1, invalid, 10.0.0.2")
	headers.Add("X-Forwarded-For", "10.0.0.4:5000")
	headers.Set("X-Real-Ip", "10.0.0.3")
	headers.Set("True-Client-Ip", "10.0.0.9")

	req := &AuthRequest{Headers: headers}

	require.Equal(
		t,
		[]netip.Addr{
			netip.MustParseAddr("10.0.0.1"),
			{},
			netip.MustParseAddr("10.0.0.2"),
			netip.MustParseAddr("10.0.0.4"),
		},
		req.ExtractForwardedFor(),
	)
	require.Equal(t, netip.MustParseAddr("10.0.0.3"), req.ExtractRealIP())
}

func TestAuthRequestExtractForwardedForFromMetadata(t *testing.T) {
	req := &AuthRequest{Headers: http.Header{"x-forwarded-for": {"1.1.1.1", "10.0.0.2"}}}

	require.Equal(t, []netip.Addr{netip.MustParseAddr("1.1.1.1"), netip.MustParseAddr("10.0.0.2")}, req.ExtractForwardedFor())
}

func TestParseBearer(t *testing.T) {
	for _, v := range []string{"Bearer abc", "bearer abc", "abc", "Bearer  abc", "Bearer\tabc", " BEARER abc "} {
		require.Equal(t, "abc", ParseBearer(v), v)
	}
	for _, v := range []string{"", "Basic abc", "Bearer a b", "Token abc"} {
		require.Empty(t, ParseBearer(v), v)
	}
}

func TestParseBasic(t *testing.T) {
	user, pass, ok := ParseBasic("basic  dTpw")
	require.True(t, ok)
	require.Equal(t, "u", user)
	require.Equal(t, "p", pass)

	for _, v := range []string{"", "Bearer dTpw", "Basic !!!", "Basic dQ=="} {
		_, _, ok = ParseBasic(v)
		require.False(t, ok, v)
	}
}
