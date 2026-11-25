package clientstat

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	blockedClientModel "github.com/rendau/ruto/internal/domain/blockedclient/model"
	"github.com/rendau/ruto/internal/domain/clientstat/model"
	sessionModel "github.com/rendau/ruto/internal/domain/session/model"
	"github.com/rendau/ruto/internal/errs"
)

type testSession struct {
	session *sessionModel.Session
}

func (s *testSession) FromContext(_ context.Context) *sessionModel.Session { return s.session }

type testService struct {
	added      []*model.ClientStat
	receivedAt time.Time
	window     *model.Window
	seriesIp   string
	topReq     *model.TopReq
	top        []*model.Item
	deleted    time.Time
}

func (s *testService) Add(_ context.Context, receivedAt time.Time, items []*model.ClientStat) error {
	s.receivedAt = receivedAt
	s.added = append(s.added, items...)
	return nil
}

func (s *testService) Overview(_ context.Context, w *model.Window) (*model.Overview, error) {
	s.window = w
	return &model.Overview{Step: w.Step, Totals: &model.Totals{}}, nil
}

func (s *testService) Series(_ context.Context, w *model.Window, ip string) (*model.Series, error) {
	s.window = w
	s.seriesIp = ip
	return &model.Series{Step: w.Step}, nil
}

func (s *testService) Top(_ context.Context, req *model.TopReq) ([]*model.Item, error) {
	s.topReq = req
	return s.top, nil
}

func (s *testService) DeleteBefore(_ context.Context, before time.Time) (int64, error) {
	s.deleted = before
	return 3, nil
}

type testBlocked struct {
	items []*blockedClientModel.BlockedClient
	pars  *blockedClientModel.ListReq
}

func (s *testBlocked) List(_ context.Context, pars *blockedClientModel.ListReq) ([]*blockedClientModel.BlockedClient, int64, error) {
	s.pars = pars
	return s.items, int64(len(s.items)), nil
}

var testNow = time.Date(2026, 11, 25, 12, 1, 0, 0, time.UTC)

func newTestUsecase(session *sessionModel.Session) (*Usecase, *testService, *testBlocked) {
	svc := &testService{}
	blocked := &testBlocked{}
	uc := New(svc, blocked, &testSession{session: session})
	uc.now = func() time.Time { return testNow }
	return uc, svc, blocked
}

func admin() *sessionModel.Session { return &sessionModel.Session{Id: 1, Admin: true} }

func TestUsecase_Auth(t *testing.T) {
	ctx := context.Background()

	cases := []struct {
		name    string
		session *sessionModel.Session
		err     error
	}{
		{"no session", nil, errs.NotAuthorized},
		{"anonymous", &sessionModel.Session{}, errs.NotAuthorized},
		{"not an admin", &sessionModel.Session{Id: 2, AllApps: true}, errs.NoPermission},
		{"admin", admin(), nil},
	}
	for _, c := range cases {
		uc, _, _ := newTestUsecase(c.session)

		_, err := uc.Overview(ctx, 0)
		assertErr(t, c.err, err, c.name)
		_, err = uc.Top(ctx, &TopReq{})
		assertErr(t, c.err, err, c.name)
		_, err = uc.Series(ctx, "203.0.113.7", 0)
		assertErr(t, c.err, err, c.name)
	}
}

func assertErr(t *testing.T, want, got error, msg string) {
	t.Helper()
	if want == nil {
		assert.NoError(t, got, msg)
		return
	}
	assert.ErrorIs(t, got, want, msg)
}

func TestUsecase_Report(t *testing.T) {
	// gateways report without a session
	uc, svc, _ := newTestUsecase(nil)

	err := uc.Report(context.Background(), []*model.ClientStat{
		nil,
		{Ip: "203.0.113.7", Requests: 3},
		{Ip: "", Requests: 10},
		{Ip: "198.51.100.1", Blocked: 1},
		{Ip: "198.51.100.2"},
		{Ip: "198.51.100.3", Requests: -1, Limited: 5},
		{Ip: string(make([]byte, maxFieldLen+1)), Requests: 1},
	})
	require.NoError(t, err)
	require.Len(t, svc.added, 3)
	assert.Equal(t, []string{"203.0.113.7", "", "198.51.100.1"}, []string{svc.added[0].Ip, svc.added[1].Ip, svc.added[2].Ip})
	assert.Equal(t, testNow, svc.receivedAt)

	svc.added = nil
	require.NoError(t, uc.Report(context.Background(), nil))
	assert.Nil(t, svc.added)
}

func TestUsecase_Ranges(t *testing.T) {
	ctx := context.Background()
	uc, svc, _ := newTestUsecase(admin())

	_, err := uc.Overview(ctx, 0)
	require.NoError(t, err)
	assert.Equal(t, model.NewWindow(testNow, model.DefaultRange), svc.window)

	_, err = uc.Series(ctx, " 2001:db8:1:2::/64 ", 30*24*time.Hour)
	require.NoError(t, err)
	assert.Equal(t, model.NewWindow(testNow, model.MaxRange), svc.window)
	assert.Equal(t, "2001:db8:1:2::/64", svc.seriesIp)

	_, err = uc.Top(ctx, &TopReq{Range: time.Minute})
	require.NoError(t, err)
	assert.Equal(t, model.NewWindow(testNow, model.MinRange).From, svc.topReq.Since)
}

func TestUsecase_Top_KindAndLimit(t *testing.T) {
	ctx := context.Background()
	uc, svc, _ := newTestUsecase(admin())

	_, err := uc.Top(ctx, &TopReq{})
	require.NoError(t, err)
	assert.Equal(t, model.KindRequests, svc.topReq.Kind)
	assert.Equal(t, int64(defaultLimit), svc.topReq.Limit)

	for _, kind := range []string{model.KindBlocked, model.KindLimited, model.KindTrusted} {
		_, err = uc.Top(ctx, &TopReq{Kind: kind, Limit: 1000})
		require.NoError(t, err)
		assert.Equal(t, kind, svc.topReq.Kind)
		assert.Equal(t, int64(maxLimit), svc.topReq.Limit)
	}

	_, err = uc.Top(ctx, &TopReq{Kind: "bytes"})
	fullErr, ok := errors.AsType[errs.ErrFull](err)
	require.True(t, ok, err)
	assert.Equal(t, errs.InvalidRequest, fullErr.Err)
}

func TestUsecase_Top_AttachesBlocks(t *testing.T) {
	ctx := context.Background()
	uc, svc, blocked := newTestUsecase(admin())

	block24 := &blockedClientModel.BlockedClient{Id: "b24", Ip: "203.0.113.0/24"}
	block32 := &blockedClientModel.BlockedClient{Id: "b32", Ip: "203.0.113.7/32"}
	block48 := &blockedClientModel.BlockedClient{Id: "b48", Ip: "2001:db8:1::/48"}
	block128 := &blockedClientModel.BlockedClient{Id: "b128", Ip: "2001:db8:2:2::1/128"}
	block16 := &blockedClientModel.BlockedClient{Id: "b16", Ip: "10.1.0.0/16"}
	blocked.items = []*blockedClientModel.BlockedClient{block24, block32, block48, block128, block16, {Id: "bad", Ip: "nonsense"}}

	svc.top = []*model.Item{
		{Ip: "203.0.113.7"},       // the narrowest of /24 and /32
		{Ip: "203.0.113.8"},       // the /24
		{Ip: "2001:db8:1:2::/64"}, // inside the /48
		{Ip: "2001:db8:2:2::/64"}, // a /128 inside it does not cover the whole /64
		{Ip: "10.0.0.0/8"},        // a trusted network wider than the block
		{Ip: "198.51.100.1"},      // not blocked
		{Ip: ""},                  // the others
		{Ip: "garbage"},
	}

	items, err := uc.Top(ctx, &TopReq{Kind: model.KindBlocked})
	require.NoError(t, err)
	require.Len(t, items, len(svc.top))

	ids := make([]string, len(items))
	for i, item := range items {
		if item.Block != nil {
			ids[i] = item.Block.Id
		}
	}
	assert.Equal(t, []string{"b32", "b24", "b48", "", "", "", "", ""}, ids)

	require.NotNil(t, blocked.pars.ActiveAt)
	assert.Equal(t, testNow, *blocked.pars.ActiveAt)
}

func TestUsecase_Prune(t *testing.T) {
	uc, svc, _ := newTestUsecase(nil)
	uc.Prune(context.Background())
	assert.Equal(t, testNow.Add(-model.Retention), svc.deleted)
}
