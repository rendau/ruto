package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	appModel "github.com/rendau/ruto/internal/domain/app/model"
	endpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
	rootModel "github.com/rendau/ruto/internal/domain/root/model"
	"github.com/rendau/ruto/internal/service/gw/handler/http/request"
)

func TestNewWithRequest(t *testing.T) {
	expectedRoot := &rootModel.Root{
		BaseUrl: "https://example.com",
	}
	expectedApp := &appModel.App{
		Id:         "users",
		PathPrefix: "/users",
	}
	expected := &endpointModel.Endpoint{
		Id:     "users-list",
		Method: http.MethodGet,
		Path:   "users",
	}

	var actual *request.Request
	handler := Chain(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			actual = request.Extract(r.Context())
			require.NotNil(t, actual)
			w.WriteHeader(http.StatusNoContent)
		}),
		NewWithRequest(expectedRoot, expectedApp, expected),
	)

	req := httptest.NewRequest(http.MethodGet, "/users", nil)
	rw := httptest.NewRecorder()
	handler.ServeHTTP(rw, req)

	require.Equal(t, http.StatusNoContent, rw.Code)
	require.Same(t, expectedRoot, actual.Root)
	require.Same(t, expectedApp, actual.App)
	require.Same(t, expected, actual.Endpoint)
}

func TestEndpointFromRequest_Empty(t *testing.T) {
	req := request.Extract(httptest.NewRequest(http.MethodGet, "/", nil).Context())
	require.Nil(t, req)
}
