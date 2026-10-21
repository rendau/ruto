package api_key

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	authModel "github.com/rendau/ruto/internal/domain/auth/model"
	"github.com/rendau/ruto/internal/service/gw/service/auth/model"
)

func newRequest(header, value string) *model.AuthRequest {
	headers := http.Header{}
	headers.Set(header, value)

	req := &model.AuthRequest{}
	req.SetHttpHeader(headers)

	return req
}

func TestAuthorize(t *testing.T) {
	a := New(&authModel.AuthMethodAPIKey{
		Header: "Authorization",
		Keys:   []authModel.AuthMethodAPIKeyItem{{Name: "partner", Key: "secret-key"}},
	})

	tests := []struct {
		name  string
		value string
		want  model.Result
	}{
		{"plain key", "secret-key", model.Allowed},
		{"bearer prefix", "Bearer secret-key", model.Allowed},
		{"bearer lowercase", "bearer secret-key", model.Allowed},
		{"bearer extra spaces", "Bearer   secret-key  ", model.Allowed},
		{"wrong key", "wrong", model.InvalidCredentials},
		{"bearer wrong key", "Bearer wrong", model.InvalidCredentials},
		{"empty", "", model.NoCredentials},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, a.Authorize(newRequest("Authorization", tt.value)))
		})
	}
}
