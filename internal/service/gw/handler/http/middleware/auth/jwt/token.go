package jwt

import (
	"net/http"
	"strings"
)

func extractToken(r *http.Request) string {
	headerValue := strings.TrimSpace(r.Header.Get("Authorization"))
	if headerValue == "" {
		return ""
	}

	parts := strings.Fields(headerValue)
	if len(parts) == 0 {
		return ""
	}

	if len(parts) == 1 {
		return parts[0]
	}

	if len(parts) != 2 {
		return ""
	}

	if !strings.EqualFold(parts[0], "bearer") {
		return ""
	}

	return parts[1]
}
