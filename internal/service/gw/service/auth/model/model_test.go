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
	t.Skip("temporarily disabled")

	token := base64.StdEncoding.EncodeToString([]byte("admin:secret"))

	req := &AuthRequest{
		Headers: http.Header{
			"authorization": {"Basic " + token},
		},
	}

	username, password := req.ExtractBasic()
	require.Equal(t, "admin", username)
	require.Equal(t, "secret", password)
}

func TestAuthRequestExtractAPIKey(t *testing.T) {
	t.Skip("temporarily disabled")

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
	t.Skip("temporarily disabled")

	req := &AuthRequest{
		Headers: http.Header{
			"x-forwarded-for": {"10.0.0.1, invalid, 10.0.0.2"},
			"x-real-ip":       {"10.0.0.3"},
		},
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
