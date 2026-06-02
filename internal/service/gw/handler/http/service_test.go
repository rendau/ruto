package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	appModel "github.com/rendau/ruto/internal/domain/app/model"
	endpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
	rootModel "github.com/rendau/ruto/internal/domain/root/model"
)

func TestService_RegistersHTTPEndpoint(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/profile", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer backend.Close()

	snapshot := rootModel.NewEmpty()
	snapshot.Apps = []*appModel.App{
		{
			Active:     true,
			PathPrefix: "/account",
			Name:       "account",
			Backend: appModel.AppBackend{
				Url: backend.URL,
			},
			Endpoints: []*endpointModel.Endpoint{
				{
					Active: true,
					Type:   endpointModel.TypeHTTP,
					Method: http.MethodGet,
					Path:   "profile",
				},
			},
		},
	}
	require.NoError(t, snapshot.Normalize())

	service, err := New(snapshot, false)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/account/profile", nil)
	rec := httptest.NewRecorder()

	service.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "ok", rec.Body.String())
}
