package system

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	changeRequestModel "github.com/rendau/ruto/internal/domain/changerequest/model"
	usecaseSystem "github.com/rendau/ruto/internal/usecase/system"
)

type testUsecase struct{}

func (s *testUsecase) Gateways(_ context.Context) (*usecaseSystem.Gateways, error) {
	return &usecaseSystem.Gateways{}, nil
}

func (s *testUsecase) HasUnpublishedChanges(_ context.Context) (bool, error) {
	return false, nil
}

func (s *testUsecase) PendingChangeRequests(_ context.Context, _ time.Duration) (*changeRequestModel.PendingStats, error) {
	return &changeRequestModel.PendingStats{}, nil
}

func (s *testUsecase) RoutesVersion(_ context.Context) (string, error) {
	return "v1", nil
}

func (s *testUsecase) Routes(_ context.Context) (*usecaseSystem.Routes, error) {
	return &usecaseSystem.Routes{
		Version: "v1",
		BaseUrl: "https://api.example.kz",
		Apps: []*usecaseSystem.RouteApp{{
			Id: "app1", Name: "orders", Active: true, PathPrefix: "/orders",
			BackendUrl: "http://orders.default.svc:80", GrpcUrl: "orders:5050",
			Endpoints: []*usecaseSystem.RouteEndpoint{
				{Id: "ep1", Active: true, Type: "http", Method: "GET", Path: "order/{id}"},
				{Id: "ep2", Active: true, Type: "grpc", GrpcPath: "/orders.Orders/Get"},
			},
		}},
	}, nil
}

// pulse reads the routes in the shape it used to read from the snapshot.
func TestRoutesShape(t *testing.T) {
	mux := http.NewServeMux()
	New(&testUsecase{}).Register(mux)

	get := func(path string) map[string]any {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		require.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "application/json; charset=utf-8", rec.Header().Get("Content-Type"))
		result := map[string]any{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &result))
		return result
	}

	assert.Equal(t, map[string]any{"version": "v1"}, get("/routes/version"))

	routes := get("/routes")
	assert.Equal(t, "v1", routes["version"])
	data := routes["data"].(map[string]any)
	assert.Equal(t, "https://api.example.kz", data["base_url"])
	app := data["apps"].([]any)[0].(map[string]any)
	assert.Equal(t, map[string]any{"url": "http://orders.default.svc:80", "grpc_url": "orders:5050"}, app["backend"])
	assert.Equal(t, "/orders", app["path_prefix"])
	endpoints := app["endpoints"].([]any)
	assert.Equal(t, map[string]any{"method": "GET", "path": "order/{id}"}, endpoints[0].(map[string]any)["http"])
	assert.Equal(t, map[string]any{"path": "/orders.Orders/Get"}, endpoints[1].(map[string]any)["grpc"])
}
