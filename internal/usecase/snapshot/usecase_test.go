package snapshot

import (
	"context"
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
	setCalls int
}

func (s *testSnapshotService) GetVersion(_ context.Context) (string, error) {
	return "existing", nil
}

func (s *testSnapshotService) Get(_ context.Context) (*snapshotModel.Snapshot, error) {
	return &snapshotModel.Snapshot{}, nil
}

func (s *testSnapshotService) Set(_ context.Context, _ *snapshotModel.Snapshot) error {
	s.setCalls++
	return nil
}

type testRootService struct{}

func (s *testRootService) Get(_ context.Context) (*rootModel.Root, error) {
	return &rootModel.Root{}, nil
}

type testAppService struct{}

func (s *testAppService) List(_ context.Context, _ *appModel.ListReq) ([]*appModel.App, int64, error) {
	return nil, 0, nil
}

type testEndpointService struct{}

func (s *testEndpointService) List(_ context.Context, _ *endpointModel.ListReq) ([]*endpointModel.Endpoint, int64, error) {
	return nil, 0, nil
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
			snapshotSvc := &testSnapshotService{}
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
