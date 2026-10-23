package snapshot

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	appModel "github.com/rendau/ruto/internal/domain/app/model"
	endpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
	rootModel "github.com/rendau/ruto/internal/domain/root/model"
	sessionModel "github.com/rendau/ruto/internal/domain/session/model"
	snapshotModel "github.com/rendau/ruto/internal/domain/snapshot/model"
	"github.com/rendau/ruto/internal/errs"
)

type testSnapshotService struct {
	stored   snapshotModel.Snapshot
	setCalls int
}

func (s *testSnapshotService) GetVersion(_ context.Context) (string, error) {
	return s.stored.Hash, nil
}

func (s *testSnapshotService) Get(_ context.Context) (*snapshotModel.Snapshot, error) {
	return new(s.stored), nil
}

func (s *testSnapshotService) Set(_ context.Context, obj *snapshotModel.Snapshot) error {
	s.stored = *obj
	s.setCalls++
	return nil
}

// newStoredSnapshot is a snapshot published earlier: its hash matches its data.
func newStoredSnapshot(t *testing.T) *testSnapshotService {
	data, err := json.Marshal(&rootModel.Root{})
	require.NoError(t, err)
	return &testSnapshotService{stored: snapshotModel.Snapshot{Hash: digest(data), Data: data}}
}

type testRootService struct{}

func (s *testRootService) Get(_ context.Context) (*rootModel.Root, error) {
	root := rootModel.NewEmpty()
	root.Variables = map[string]string{"TOKEN": "root-token"}
	return root, nil
}

type testAppService struct{}

func (s *testAppService) List(_ context.Context, _ *appModel.ListReq) ([]*appModel.App, int64, error) {
	return []*appModel.App{{
		Id:            "app-1",
		Active:        true,
		Name:          "shop",
		PathPrefix:    "/shop",
		AllowWildcard: true,
		Backend:       appModel.Backend{Url: "http://shop:8080"},
	}}, 1, nil
}

// testEndpointService serves the config of one app; trackPaths stands for the
// migration that adds the flag to wildcard endpoints.
type testEndpointService struct {
	trackPaths bool
}

func (s *testEndpointService) List(_ context.Context, _ *endpointModel.ListReq) ([]*endpointModel.Endpoint, int64, error) {
	return []*endpointModel.Endpoint{
		{
			Id:         "ep-wild",
			AppId:      "app-1",
			Active:     true,
			TrackPaths: s.trackPaths,
			Type:       endpointModel.TypeHTTP,
			Http:       endpointModel.Http{Method: "*", Path: "*"},
		},
		{
			Id:      "ep-exact",
			AppId:   "app-1",
			Active:  true,
			Type:    endpointModel.TypeHTTP,
			Http:    endpointModel.Http{Method: "GET", Path: "product/{code}"},
			Backend: endpointModel.Backend{Headers: map[string]string{"X-Token": "{{TOKEN}}"}},
		},
	}, 2, nil
}

type testGatewaysNotifier struct {
	calls int
}

func (s *testGatewaysNotifier) NotifyAll() {
	s.calls++
}

type testSessionService struct {
	session *sessionModel.Session
}

func (s *testSessionService) CtxIsAuthorized(_ context.Context) bool {
	return s.session.IsAuthorized()
}

func TestUsecase_Deploy(t *testing.T) {
	tests := []struct {
		name    string
		session *sessionModel.Session
		wantErr error
	}{
		{name: "anonymous", session: sessionModel.New(0), wantErr: errs.NotAuthorized},
		{name: "user", session: &sessionModel.Session{Id: 2}},
		{name: "admin", session: &sessionModel.Session{Id: 1, Admin: true}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snapshotSvc := newStoredSnapshot(t)
			notifier := &testGatewaysNotifier{}
			uc := New(snapshotSvc, &testRootService{}, &testAppService{}, &testEndpointService{}, notifier, &testSessionService{session: tt.session})

			err := uc.Deploy(context.Background())
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				require.Zero(t, snapshotSvc.setCalls)
				require.Zero(t, notifier.calls)
				return
			}

			require.NoError(t, err)
			require.Equal(t, 1, snapshotSvc.setCalls)
			require.Equal(t, 1, notifier.calls)
		})
	}
}

func TestUsecase_EnsureKeepsPublishedVersion(t *testing.T) {
	snapshotSvc := newStoredSnapshot(t)
	notifier := &testGatewaysNotifier{}
	session := &testSessionService{session: &sessionModel.Session{Id: 1, Admin: true}}

	uc := New(snapshotSvc, &testRootService{}, &testAppService{}, &testEndpointService{}, notifier, session)
	require.Zero(t, snapshotSvc.setCalls)

	require.NoError(t, uc.Deploy(context.Background()))
	published := snapshotSvc.stored.Hash

	// a core restart reads the snapshot it published itself
	New(snapshotSvc, &testRootService{}, &testAppService{}, &testEndpointService{}, notifier, session)
	require.Equal(t, 1, snapshotSvc.setCalls)
	require.Equal(t, published, snapshotSvc.stored.Hash)
}

func TestUsecase_EnsureRecomputesEditedSnapshot(t *testing.T) {
	snapshotSvc := newStoredSnapshot(t)
	notifier := &testGatewaysNotifier{}
	session := &testSessionService{session: &sessionModel.Session{Id: 1, Admin: true}}

	uc := New(snapshotSvc, &testRootService{}, &testAppService{}, &testEndpointService{}, notifier, session)
	require.NoError(t, uc.Deploy(context.Background()))
	published := snapshotSvc.stored.Hash

	// the migration sets the flag in the config and in the published snapshot,
	// leaving the hash as it was
	var data map[string]any
	require.NoError(t, json.Unmarshal(snapshotSvc.stored.Data, &data))
	wildcard := data["apps"].([]any)[0].(map[string]any)["endpoints"].([]any)[0].(map[string]any)
	require.Equal(t, "ep-wild", wildcard["id"])
	wildcard["track_paths"] = true
	edited, err := json.Marshal(data)
	require.NoError(t, err)
	snapshotSvc.stored.Data = edited

	migrated := &testEndpointService{trackPaths: true}
	uc = New(snapshotSvc, &testRootService{}, &testAppService{}, migrated, notifier, session)

	require.Equal(t, 2, snapshotSvc.setCalls)
	require.NotEqual(t, published, snapshotSvc.stored.Hash, "gateways must see a new version")

	// nothing is left unpublished: the config builds into the same version
	current, err := uc.CurrentVersion(context.Background())
	require.NoError(t, err)
	require.Equal(t, current, snapshotSvc.stored.Hash)
}
