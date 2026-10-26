package system

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appModel "github.com/rendau/ruto/internal/domain/app/model"
	authModel "github.com/rendau/ruto/internal/domain/auth/model"
	changeRequestModel "github.com/rendau/ruto/internal/domain/changerequest/model"
	endpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
	rootModel "github.com/rendau/ruto/internal/domain/root/model"
	snapshotModel "github.com/rendau/ruto/internal/domain/snapshot/model"
	varsModel "github.com/rendau/ruto/internal/domain/vars/model"
	usecaseGateway "github.com/rendau/ruto/internal/usecase/gateway"
)

type testSnapshotService struct {
	stored snapshotModel.Snapshot
}

func (s *testSnapshotService) GetVersion(_ context.Context) (string, error) {
	return s.stored.Hash, nil
}

func (s *testSnapshotService) Get(_ context.Context) (*snapshotModel.Snapshot, error) {
	return new(s.stored), nil
}

type testSnapshotUsecase struct {
	current string
}

func (s *testSnapshotUsecase) CurrentVersion(_ context.Context) (string, error) {
	return s.current, nil
}

type testGatewayUsecase struct {
	items []*usecaseGateway.Item
}

func (s *testGatewayUsecase) ListAll(_ context.Context) ([]*usecaseGateway.Item, error) {
	return s.items, nil
}

type testGatewaysPool struct {
	ids []string
}

func (s *testGatewaysPool) ConnectedIDs() []string {
	return s.ids
}

type testChangeRequestService struct{}

func (s *testChangeRequestService) PendingStats(_ context.Context, _ time.Duration) (*changeRequestModel.PendingStats, error) {
	return &changeRequestModel.PendingStats{}, nil
}

func newTestUsecase(snapshot snapshotModel.Snapshot, current string, items []*usecaseGateway.Item, connected []string) *Usecase {
	return New(
		&testSnapshotService{stored: snapshot},
		&testSnapshotUsecase{current: current},
		&testGatewayUsecase{items: items},
		&testGatewaysPool{ids: connected},
		&testChangeRequestService{},
	)
}

// Every secret position of the configuration holds a marked value: none of
// them may reach the routes.
func TestRoutesDropSecrets(t *testing.T) {
	auth := authModel.Auth{Enabled: true, Methods: []*authModel.AuthMethod{
		{Basic: &authModel.AuthMethodBasic{Users: []authModel.AuthMethodBasicUser{{Username: "u", Password: "SECRET-basic"}}}},
		{APIKey: &authModel.AuthMethodAPIKey{Keys: []authModel.AuthMethodAPIKeyItem{{Name: "k", Key: "SECRET-apikey"}}}},
	}}
	root := &rootModel.Root{
		BaseUrl:   "https://api.example.kz",
		Auth:      auth,
		Variables: varsModel.Vars{"TOKEN": "SECRET-root-var"},
		Apps: []*appModel.App{{
			Id:         "app1",
			Name:       "orders",
			Active:     true,
			PathPrefix: "/orders",
			Auth:       auth,
			Variables:  varsModel.Vars{"KEY": "SECRET-app-var"},
			Backend: appModel.Backend{
				Url:         "http://user:SECRET-url-pass@orders.default.svc:80/api?token=SECRET-url-query",
				GrpcUrl:     "user:SECRET-grpc-pass@orders:5050",
				Headers:     varsModel.Vars{"Authorization": "SECRET-app-header"},
				QueryParams: varsModel.Vars{"key": "SECRET-app-query"},
			},
			Endpoints: []*endpointModel.Endpoint{{
				Id:        "ep1",
				Active:    true,
				Type:      endpointModel.TypeHTTP,
				Http:      endpointModel.Http{Method: "GET", Path: "order/{id}"},
				Auth:      auth,
				Variables: varsModel.Vars{"EP": "SECRET-ep-var"},
				Backend: endpointModel.Backend{
					Headers:     varsModel.Vars{"X-Key": "SECRET-ep-header"},
					QueryParams: varsModel.Vars{"k": "SECRET-ep-query"},
				},
			}},
		}},
	}
	data, err := json.Marshal(root)
	require.NoError(t, err)
	require.Contains(t, string(data), "SECRET-") // the fixture really carries the secrets

	uc := newTestUsecase(snapshotModel.Snapshot{Hash: "v1", Data: data}, "v1", nil, nil)

	routes, err := uc.Routes(context.Background())
	require.NoError(t, err)

	out, err := json.Marshal(routes)
	require.NoError(t, err)
	assert.NotContains(t, string(out), "SECRET")

	require.Len(t, routes.Apps, 1)
	app := routes.Apps[0]
	assert.Equal(t, "v1", routes.Version)
	assert.Equal(t, "https://api.example.kz", routes.BaseUrl)
	assert.Equal(t, "http://orders.default.svc:80/api", app.BackendUrl)
	assert.Equal(t, "orders:5050", app.GrpcUrl)
	require.Len(t, app.Endpoints, 1)
	assert.Equal(t, &RouteEndpoint{Id: "ep1", Active: true, Type: "http", Method: "GET", Path: "order/{id}"}, app.Endpoints[0])
}

func TestSafeAddress(t *testing.T) {
	for in, want := range map[string]string{
		"":                                "",
		"http://orders.default.svc:80":    "http://orders.default.svc:80",
		"https://u:p@host/api?key=x#frag": "https://host/api",
		"http://{{HOST}}:80":              "",
		"orders:5050":                     "orders:5050",
		"dns:///orders:5050":              "dns:///orders:5050",
		"u:p@orders:5050":                 "orders:5050",
		"orders:5050?token=x":             "orders:5050",
	} {
		assert.Equal(t, want, safeAddress(in), in)
	}
}

func TestGateways(t *testing.T) {
	now := time.Now().Unix()
	items := []*usecaseGateway.Item{
		{GatewayID: "gw-a-1", HostName: "gw-a", Status: usecaseGateway.StatusOnline, SnapshotVersion: "v2", LastSeenAtUnix: now},
		{GatewayID: "gw-b-1", HostName: "gw-b", Status: usecaseGateway.StatusOnline, SnapshotVersion: "v1", LastSeenAtUnix: now, LastError: "boom"},
		{GatewayID: "gw-c-1", HostName: "gw-c", Status: usecaseGateway.StatusOffline, SnapshotVersion: "v2"},
		{GatewayID: "gw-d-1", HostName: "gw-d", Status: usecaseGateway.StatusOffline, SnapshotVersion: "v1"},
	}
	uc := newTestUsecase(snapshotModel.Snapshot{Hash: "v2"}, "v3", items, []string{"gw-a-1", "gw-c-1", "gw-e-1"})
	ctx := context.Background()

	byHost := func(g *Gateways) map[string]*Gateway {
		result := map[string]*Gateway{}
		for _, gw := range g.Gateways {
			result[gw.HostName] = gw
		}
		return result
	}

	// just published: gw-b may still be applying it
	gateways, err := uc.Gateways(ctx)
	require.NoError(t, err)
	assert.Equal(t, "v2", gateways.PublishedVersion)
	got := byHost(gateways)
	require.Len(t, got, 5)
	assert.False(t, got["gw-b"].Lagging)
	assert.False(t, got["gw-b"].UpToDate)
	assert.True(t, got["gw-a"].UpToDate)

	// the publish is older than LagAfter
	uc.publishedVersionAt = time.Now().Add(-LagAfter - time.Second)
	gateways, err = uc.Gateways(ctx)
	require.NoError(t, err)
	got = byHost(gateways)
	assert.True(t, got["gw-b"].Lagging, "online on an old version")
	assert.False(t, got["gw-d"].Lagging, "offline is not lagging")
	assert.Equal(t, "boom", got["gw-b"].LastError)

	assert.False(t, got["gw-a"].Silent)
	assert.True(t, got["gw-c"].Silent, "connected, but its heartbeats stopped")
	assert.False(t, got["gw-d"].Silent, "gone: neither heartbeats nor a stream")
	assert.True(t, got["gw-e-1"].Silent, "connected, never reported")
	assert.True(t, got["gw-e-1"].Connected)

	has, err := uc.HasUnpublishedChanges(ctx)
	require.NoError(t, err)
	assert.True(t, has)
}
