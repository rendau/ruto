package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	localContext "github.com/rendau/ruto/internal/service/gw/handler/http/context"
	"github.com/rendau/ruto/internal/service/gw/model/config"

	"github.com/stretchr/testify/require"
)

func TestNewWithEndpoint(t *testing.T) {
	expected := &config.Endpoint{
		Id:     "users-list",
		Method: http.MethodGet,
		Path:   "users",
	}

	var actual *config.Endpoint
	handler := Chain(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			endpoint := localContext.ExtractEndpoint(r.Context())
			require.NotNil(t, endpoint)
			actual = endpoint
			w.WriteHeader(http.StatusNoContent)
		}),
		NewWithEndpoint(expected),
	)

	req := httptest.NewRequest(http.MethodGet, "/users", nil)
	rw := httptest.NewRecorder()
	handler.ServeHTTP(rw, req)

	require.Equal(t, http.StatusNoContent, rw.Code)
	require.Same(t, expected, actual)
}

func TestEndpointFromRequest_Empty(t *testing.T) {
	endpoint := localContext.ExtractEndpoint(httptest.NewRequest(http.MethodGet, "/", nil).Context())
	require.Nil(t, endpoint)
}
