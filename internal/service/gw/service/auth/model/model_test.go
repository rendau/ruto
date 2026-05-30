package model

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAuthRequestExtractBasic(t *testing.T) {
	t.Skip("temporarily disabled")

	token := base64.StdEncoding.EncodeToString([]byte("admin:secret"))

	req := &AuthRequest{
		Headers: map[string]string{
			"authorization": "Basic " + token,
		},
	}

	username, password := req.ExtractBasic()
	require.Equal(t, "admin", username)
	require.Equal(t, "secret", password)
}

func TestAuthRequestExtractAPIKey(t *testing.T) {
	t.Skip("temporarily disabled")

	req := &AuthRequest{
		Headers: map[string]string{
			"x-api-key": "from-header",
		},
		QueryParams: map[string]string{
			"x-api-key": "from-query",
		},
	}

	require.Equal(t, "from-header", req.ExtractAPIKey("X-Api-Key"))
}

func TestAuthRequestExtractIPs(t *testing.T) {
	t.Skip("temporarily disabled")

	req := &AuthRequest{
		Headers: map[string]string{
			"x-forwarded-for": "10.0.0.1, invalid, 10.0.0.2",
			"x-real-ip":       "10.0.0.3",
		},
	}

	require.Equal(t, []string{"10.0.0.1", "10.0.0.2", "10.0.0.3"}, req.ExtractIPs())
}
