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

func TestAuthRequestExtractIPAddrs(t *testing.T) {
	headers := http.Header{}
	headers.Set("X-Forwarded-For", "10.0.0.1, invalid, 10.0.0.2")
	headers.Set("X-Real-Ip", "10.0.0.3")

	req := &AuthRequest{
		Headers: headers,
	}

	require.Equal(
		t,
		[]netip.Addr{
			netip.MustParseAddr("10.0.0.1"),
			netip.MustParseAddr("10.0.0.2"),
			netip.MustParseAddr("10.0.0.3"),
		},
		req.ExtractIPAddrs(),
	)
}
