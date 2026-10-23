package seenpath

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/rendau/ruto/internal/domain/seenpath/model"
	"github.com/rendau/ruto/internal/errs"
)

type testSession struct {
	authorized bool
	fullAccess bool
	appIds     []string
}

func (s *testSession) CtxIsAuthorized(_ context.Context) bool     { return s.authorized }
func (s *testSession) CtxHasFullAppAccess(_ context.Context) bool { return s.fullAccess }
func (s *testSession) CtxGetAppIds(_ context.Context) []string    { return s.appIds }

type testService struct {
	added        []*model.SeenPath
	deleted      []string
	deletedItems map[string][]*model.SeenPathKey
}

func (s *testService) Add(_ context.Context, items []*model.SeenPath) error {
	s.added = append(s.added, items...)
	return nil
}

func (s *testService) ListByApp(_ context.Context, appId string) ([]*model.SeenPath, error) {
	return []*model.SeenPath{{AppId: appId, Path: "docs/{id}"}}, nil
}

func (s *testService) DeleteItems(_ context.Context, appId string, keys []*model.SeenPathKey) error {
	if s.deletedItems == nil {
		s.deletedItems = map[string][]*model.SeenPathKey{}
	}
	s.deletedItems[appId] = append(s.deletedItems[appId], keys...)
	return nil
}

func (s *testService) DeleteByApp(_ context.Context, appId string) error {
	s.deleted = append(s.deleted, appId)
	return nil
}

func TestUsecase_Report_SkipsBrokenItems(t *testing.T) {
	svc := &testService{}
	uc := New(svc, &testSession{})

	err := uc.Report(context.Background(), []*model.SeenPath{
		nil,
		{AppId: "app-1", EndpointId: "ep-1", Method: "GET", Path: "docs/{id}", Hits: 3},
		{AppId: "", EndpointId: "ep-1", Method: "GET", Path: "x", Hits: 1},
		{AppId: "app-1", EndpointId: "ep-1", Method: "GET", Path: "y", Hits: 0},
	})
	require.NoError(t, err)
	require.Len(t, svc.added, 1)
	require.Equal(t, "docs/{id}", svc.added[0].Path)
}

func TestUsecase_List(t *testing.T) {
	uc := New(&testService{}, &testSession{})
	_, err := uc.List(context.Background(), "app-1")
	require.ErrorIs(t, err, errs.NotAuthorized)

	// a viewer without access to the app may still read the report
	uc = New(&testService{}, &testSession{authorized: true})
	items, err := uc.List(context.Background(), " app-1 ")
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, "app-1", items[0].AppId)

	_, err = uc.List(context.Background(), " ")
	require.ErrorIs(t, err, errs.IdRequired)
}

func TestUsecase_Clear_NeedsAppAccess(t *testing.T) {
	svc := &testService{}

	err := New(svc, &testSession{authorized: true}).Clear(context.Background(), "app-1")
	require.ErrorIs(t, err, errs.NoPermission)
	require.Empty(t, svc.deleted)

	require.NoError(t, New(svc, &testSession{authorized: true, appIds: []string{"app-1"}}).Clear(context.Background(), "app-1"))
	require.NoError(t, New(svc, &testSession{authorized: true, fullAccess: true}).Clear(context.Background(), "app-2"))
	require.Equal(t, []string{"app-1", "app-2"}, svc.deleted)
}

func TestUsecase_Delete(t *testing.T) {
	svc := &testService{}
	key := &model.SeenPathKey{EndpointId: "ep-1", Method: "GET", Path: "product/{id}/reviews"}

	err := New(svc, &testSession{}).Delete(context.Background(), "app-1", []*model.SeenPathKey{key})
	require.ErrorIs(t, err, errs.NotAuthorized)

	err = New(svc, &testSession{authorized: true, appIds: []string{"app-2"}}).Delete(context.Background(), "app-1", []*model.SeenPathKey{key})
	require.ErrorIs(t, err, errs.NoPermission)

	err = New(svc, &testSession{authorized: true, fullAccess: true}).Delete(context.Background(), " ", []*model.SeenPathKey{key})
	require.ErrorIs(t, err, errs.IdRequired)
	require.Empty(t, svc.deletedItems)

	uc := New(svc, &testSession{authorized: true, appIds: []string{"app-1"}})
	require.NoError(t, uc.Delete(context.Background(), " app-1 ", []*model.SeenPathKey{
		nil,
		key,
		{EndpointId: "", Method: "GET", Path: "x"},
		{EndpointId: "ep-1", Method: "", Path: "y"},
	}))
	require.Equal(t, map[string][]*model.SeenPathKey{"app-1": {key}}, svc.deletedItems)

	// nothing valid left: the service is not called
	require.NoError(t, uc.Delete(context.Background(), "app-1", []*model.SeenPathKey{nil}))
	require.Len(t, svc.deletedItems["app-1"], 1)
}
