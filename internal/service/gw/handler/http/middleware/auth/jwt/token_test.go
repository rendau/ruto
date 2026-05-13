package jwt

import (
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtractToken(t *testing.T) {
	tests := []struct {
		name      string
		header    string
		wantToken string
	}{
		{
			name:      "bearer prefix",
			header:    "Bearer token-1",
			wantToken: "token-1",
		},
		{
			name:      "without prefix",
			header:    "token-2",
			wantToken: "token-2",
		},
		{
			name:      "bearer lower case with extra spaces",
			header:    "   bearer    token-3   ",
			wantToken: "token-3",
		},
		{
			name:      "unsupported scheme",
			header:    "Basic abc",
			wantToken: "",
		},
		{
			name:      "malformed bearer",
			header:    "Bearer token-1 extra",
			wantToken: "",
		},
		{
			name:      "empty header",
			header:    "",
			wantToken: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "http://localhost", nil)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}

			got := extractToken(req)
			require.Equal(t, tt.wantToken, got)
		})
	}
}
