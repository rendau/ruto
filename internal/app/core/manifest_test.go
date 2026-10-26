package core

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
	handlerSystemP "github.com/rendau/ruto/internal/handler/system"
	usecaseGateway "github.com/rendau/ruto/internal/usecase/gateway"
	usecaseSystem "github.com/rendau/ruto/internal/usecase/system"
)

type testSystemUsecase struct {
	gateways *usecaseSystem.Gateways
}

func (s *testSystemUsecase) Gateways(_ context.Context) (*usecaseSystem.Gateways, error) {
	return s.gateways, nil
}

func (s *testSystemUsecase) HasUnpublishedChanges(_ context.Context) (bool, error) {
	return true, nil
}

func (s *testSystemUsecase) PendingChangeRequests(_ context.Context, stuckAfter time.Duration) (*changeRequestModel.PendingStats, error) {
	return &changeRequestModel.PendingStats{Count: 3, Stuck: 1, OldestAt: new(time.Now().Add(-stuckAfter - time.Hour))}, nil
}

func (s *testSystemUsecase) RoutesVersion(_ context.Context) (string, error) {
	return "", nil
}

func (s *testSystemUsecase) Routes(_ context.Context) (*usecaseSystem.Routes, error) {
	return &usecaseSystem.Routes{}, nil
}

func newTestSystemUsecase() *testSystemUsecase {
	at := time.Now().Add(-time.Minute)
	return &testSystemUsecase{gateways: &usecaseSystem.Gateways{
		PublishedVersion: "0123456789abcdef0123456789abcdef",
		Gateways: []*usecaseSystem.Gateway{
			{HostName: "ruto-gateway-abcde", Status: usecaseGateway.StatusOnline, Connected: true, Version: "0123456789abcdef", UpToDate: true, LastApplyAt: &at, LastSeenAt: &at},
			{HostName: "ruto-gateway-fghij", Status: usecaseGateway.StatusOnline, Connected: true, Version: "fedcba9876543210", Lagging: true, LastSeenAt: &at,
				LastError: "rpc error: code = Unavailable desc = connection error: dial tcp 10.0.0.1:5050: connect: connection refused"},
			{HostName: "ruto-gateway-klmno-7", Status: usecaseGateway.StatusOffline, Connected: true, Silent: true},
		},
	}}
}

// The manifest follows the standard: nothing is dropped, no text is over a limit.
func TestManifest(t *testing.T) {
	kit := newPulsekit()
	handlerSystemP.New(newTestSystemUsecase()).RegisterPulse(kit)

	require.Empty(t, kit.Problems())
	require.Empty(t, kit.Warnings())

	mux := http.NewServeMux()
	kit.Register(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/.well-known/pulse", nil))
	require.Equal(t, http.StatusOK, rec.Code)

	manifest := map[string]any{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &manifest))
	assert.Equal(t, "ruto-core", manifest["service"].(map[string]any)["name"])
	assert.Len(t, manifest["metrics"], 3)
	assert.Len(t, manifest["endpoints"], 1)
}

func TestGatewaysEndpoint(t *testing.T) {
	kit := newPulsekit()
	handlerSystemP.New(newTestSystemUsecase()).RegisterPulse(kit)

	require.NoError(t, kit.CheckEndpoint("gateways", nil))

	mux := http.NewServeMux()
	kit.Register(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/diag/gateways", nil))
	require.Equal(t, http.StatusOK, rec.Code)

	rep := &handlerSystemP.GatewaysRep{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), rep))
	assert.Equal(t, "0123456789ab", rep.PublishedVersion)
	assert.True(t, rep.HasUnpublishedChanges)
	require.Len(t, rep.Gateways, 3)
	assert.Empty(t, rep.Gateways[0].Problem)
	// the raw error (with an address) is replaced by a short reason
	assert.Equal(t, "в соединении отказано", rep.Gateways[1].Problem)
	assert.True(t, rep.Gateways[2].Silent)
}

// Gauges and the change request report: a lagging and a silent gateway make
// core degraded.
func TestStatus(t *testing.T) {
	kit := newPulsekit()
	handlerSystemP.New(newTestSystemUsecase()).RegisterPulse(kit)

	ctx, cancel := context.WithCancel(context.Background())
	kit.Start(ctx)
	require.Eventually(t, func() bool { return kit.Status().CheckedAt != "" }, 5*time.Second, 10*time.Millisecond)
	cancel()
	kit.Wait()

	status := kit.Status()
	assert.Equal(t, "degraded", status.Status)

	gauges := map[string]string{}
	values := map[string]any{}
	for _, g := range status.Gauges {
		gauges[g.Id] = g.Status
		values[g.Id] = g.Value
	}
	assert.Equal(t, map[string]string{
		"gateways_online": "ok", "gateways_lagging": "degraded", "gateways_silent": "degraded", "unpublished_changes": "ok",
	}, gauges)
	assert.EqualValues(t, 2, values["gateways_online"])

	require.Len(t, status.Entities, 1)
	entity := status.Entities[0]
	assert.Equal(t, handlerSystemP.ChangeRequestEntity, entity.Name)
	assert.Equal(t, "ok", entity.Status, "stuck change requests are only shown")
	require.Len(t, entity.Statuses, 1)
	assert.Equal(t, changeRequestModel.StatusPending, entity.Statuses[0].Name)
	assert.Equal(t, 3, entity.Statuses[0].Count)
	assert.Equal(t, 1, entity.Statuses[0].Stuck)
	assert.Greater(t, entity.Statuses[0].OldestS, int64(24*3600))
}
