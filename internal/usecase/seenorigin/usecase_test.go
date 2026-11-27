package seenorigin

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/rendau/ruto/internal/domain/seenorigin/model"
	"github.com/rendau/ruto/internal/errs"
)

type testSession struct {
	authorized bool
	admin      bool
}

func (s *testSession) CtxIsAuthorized(_ context.Context) bool { return s.authorized }
func (s *testSession) CtxIsAdmin(_ context.Context) bool      { return s.admin }

type testService struct {
	added    []*model.SeenOrigin
	addedAt  time.Time
	listedAt time.Time
	prunedAt time.Time
	deleted  []string
	cleared  int
}

func (s *testService) Add(_ context.Context, now time.Time, items []*model.SeenOrigin) error {
	s.addedAt = now
	s.added = append(s.added, items...)
	return nil
}

func (s *testService) List(_ context.Context, now time.Time) ([]*model.SeenOrigin, error) {
	s.listedAt = now
	return []*model.SeenOrigin{{Origin: "https://app.example.com", Hits: 5}}, nil
}

func (s *testService) Prune(_ context.Context, now time.Time) (int64, error) {
	s.prunedAt = now
	return 0, nil
}

func (s *testService) DeleteItems(_ context.Context, origins []string) error {
	s.deleted = append(s.deleted, origins...)
	return nil
}

func (s *testService) DeleteAll(_ context.Context) error {
	s.cleared++
	return nil
}

func TestUsecase_Report_SkipsBrokenItems(t *testing.T) {
	svc := &testService{}
	uc := New(svc, &testSession{})

	err := uc.Report(context.Background(), []*model.SeenOrigin{
		nil,
		{Origin: "https://app.example.com", Hits: 3},
		{Origin: "", Hits: 1},
		{Origin: "https://b.example.com", Hits: 0},
	})
	require.NoError(t, err)
	require.Len(t, svc.added, 1)
	require.Equal(t, "https://app.example.com", svc.added[0].Origin)
}

func TestUsecase_List(t *testing.T) {
	uc := New(&testService{}, &testSession{})
	_, err := uc.List(context.Background())
	require.ErrorIs(t, err, errs.NotAuthorized)

	// any logged-in user may read the report
	uc = New(&testService{}, &testSession{authorized: true})
	items, err := uc.List(context.Background())
	require.NoError(t, err)
	require.Len(t, items, 1)
}

func TestUsecase_DeleteAndClear_AdminOnly(t *testing.T) {
	svc := &testService{}
	uc := New(svc, &testSession{authorized: true})
	require.ErrorIs(t, uc.Delete(context.Background(), []string{"https://a.example"}), errs.NoPermission)
	require.ErrorIs(t, uc.Clear(context.Background()), errs.NoPermission)

	uc = New(svc, &testSession{})
	require.ErrorIs(t, uc.Clear(context.Background()), errs.NotAuthorized)

	uc = New(svc, &testSession{authorized: true, admin: true})
	require.NoError(t, uc.Delete(context.Background(), []string{" https://a.example ", "", "https://b.example"}))
	require.Equal(t, []string{"https://a.example", "https://b.example"}, svc.deleted)

	require.NoError(t, uc.Delete(context.Background(), []string{" "}))
	require.Len(t, svc.deleted, 2)

	require.NoError(t, uc.Clear(context.Background()))
	require.Equal(t, 1, svc.cleared)
}

func TestUsecase_UsesItsClock(t *testing.T) {
	now := time.Date(2026, 11, 27, 10, 0, 0, 0, time.UTC)
	svc := &testService{}
	uc := New(svc, &testSession{authorized: true})
	uc.now = func() time.Time { return now }

	require.NoError(t, uc.Report(context.Background(), []*model.SeenOrigin{{Origin: "https://a.example", Hits: 1}}))
	_, err := uc.List(context.Background())
	require.NoError(t, err)
	uc.Prune(context.Background())

	require.Equal(t, now, svc.addedAt)
	require.Equal(t, now, svc.listedAt)
	require.Equal(t, now, svc.prunedAt)
}
