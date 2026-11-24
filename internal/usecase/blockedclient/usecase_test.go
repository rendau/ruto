package blockedclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rendau/ruto/internal/domain/blockedclient/model"
	"github.com/rendau/ruto/internal/domain/blockedclient/service"
	rootModel "github.com/rendau/ruto/internal/domain/root/model"
	sessionModel "github.com/rendau/ruto/internal/domain/session/model"
	usrModel "github.com/rendau/ruto/internal/domain/usr/model"
	"github.com/rendau/ruto/internal/errs"
)

// testService keeps blocks in memory with the same semantics as the repo.
type testService struct {
	items  []*model.BlockedClient
	seq    int
	locked int
}

func (s *testService) List(_ context.Context, pars *model.ListReq) ([]*model.BlockedClient, int64, error) {
	result := lo.Filter(s.items, func(v *model.BlockedClient, _ int) bool {
		switch {
		case pars.Ip != nil && v.Ip != *pars.Ip:
			return false
		case pars.ActiveAt != nil && !v.IsActive(*pars.ActiveAt):
			return false
		case pars.InactiveAt != nil && v.IsActive(*pars.InactiveAt):
			return false
		case pars.NotRemoved && v.RemovedAt != nil:
			return false
		case pars.Source != nil && v.Source != *pars.Source:
			return false
		case pars.ApiKeyId != nil && v.ApiKeyId != *pars.ApiKeyId:
			return false
		case pars.CreatedAfter != nil && v.CreatedAt.Before(*pars.CreatedAfter):
			return false
		case pars.RemovedSource != nil && v.RemovedSource != *pars.RemovedSource:
			return false
		case pars.RemovedAfter != nil && (v.RemovedAt == nil || !v.RemovedAt.After(*pars.RemovedAfter)):
			return false
		}
		return true
	})
	if pars.PageSize > 0 && int64(len(result)) > pars.PageSize {
		result = result[:pars.PageSize]
	}
	return lo.Map(result, func(v *model.BlockedClient, _ int) *model.BlockedClient { return new(*v) }), int64(len(result)), nil
}

func (s *testService) Get(_ context.Context, id string, errNE bool) (*model.BlockedClient, bool, error) {
	for _, v := range s.items {
		if v.Id == id {
			return new(*v), true, nil
		}
	}
	if errNE {
		return nil, false, errs.ObjectNotFound
	}
	return nil, false, nil
}

func (s *testService) GetNotRemoved(ctx context.Context, ip string) (*model.BlockedClient, bool, error) {
	items, _, _ := s.List(ctx, &model.ListReq{Ip: &ip, NotRemoved: true})
	if len(items) == 0 {
		return nil, false, nil
	}
	return items[0], true, nil
}

func (s *testService) CountActive(ctx context.Context, now time.Time, source *string) (int64, error) {
	_, n, _ := s.List(ctx, &model.ListReq{ActiveAt: &now, Source: source})
	return n, nil
}

func (s *testService) CountCreatedSince(ctx context.Context, source string, since time.Time) (int64, error) {
	_, n, _ := s.List(ctx, &model.ListReq{Source: &source, CreatedAfter: &since})
	return n, nil
}

func (s *testService) ListLiftedByAdmin(ctx context.Context, since time.Time) ([]*model.BlockedClient, error) {
	items, _, _ := s.List(ctx, &model.ListReq{RemovedSource: new(model.SourceAdmin), RemovedAfter: &since})
	return items, nil
}

func (s *testService) RemoveMany(_ context.Context, req *model.RemoveMany) (int64, error) {
	var n int64
	for _, v := range s.items {
		if v.Source != model.SourceAgent || !v.IsActive(req.Now) || (req.ApiKeyId != nil && v.ApiKeyId != *req.ApiKeyId) {
			continue
		}
		v.RemovedAt, v.RemovedBy, v.RemoveReason, v.RemovedSource = new(req.Now), req.RemovedBy, req.RemoveReason, req.RemovedSource
		n++
	}
	return n, nil
}

func (s *testService) Create(_ context.Context, obj *model.BlockedClient) (string, error) {
	for _, v := range s.items {
		if v.Ip == obj.Ip && v.RemovedAt == nil {
			return "", errors.New("unique violation")
		}
	}
	s.seq++
	item := new(*obj)
	item.Id = strconv.Itoa(s.seq)
	item.UpdatedAt = item.CreatedAt
	s.items = append(s.items, item)
	return item.Id, nil
}

func (s *testService) Update(_ context.Context, id string, e *model.Edit) error {
	for _, v := range s.items {
		if v.Id != id {
			continue
		}
		if e.Reason != nil {
			v.Reason = *e.Reason
		}
		if e.UpdatedAt != nil {
			v.UpdatedAt = *e.UpdatedAt
		}
		if e.Source != nil {
			v.Source = *e.Source
		}
		if e.SetExpiresAt {
			v.ExpiresAt = e.ExpiresAt
		}
		if e.RemovedAt != nil {
			v.RemovedAt = e.RemovedAt
		}
		if e.RemovedBy != nil {
			v.RemovedBy = *e.RemovedBy
		}
		if e.RemoveReason != nil {
			v.RemoveReason = *e.RemoveReason
		}
		if e.RemovedSource != nil {
			v.RemovedSource = *e.RemovedSource
		}
	}
	return nil
}

func (s *testService) Lock(_ context.Context) error {
	s.locked++
	return nil
}

func (s *testService) CloseExpired(_ context.Context, ip string, now time.Time) error {
	for _, v := range s.items {
		if v.Ip == ip && v.RemovedAt == nil && v.ExpiresAt != nil && !v.ExpiresAt.After(now) {
			v.RemovedAt = v.ExpiresAt
		}
	}
	return nil
}

func (s *testService) Active(ctx context.Context, now time.Time) ([]*model.Rule, string, error) {
	items, _, _ := s.List(ctx, &model.ListReq{ActiveAt: &now})
	slices.SortFunc(items, func(a, b *model.BlockedClient) int { return compareStr(a.Ip, b.Ip) })
	rules := lo.Map(items, func(v *model.BlockedClient, _ int) *model.Rule { return &model.Rule{Ip: v.Ip, ExpiresAt: v.ExpiresAt} })
	return rules, service.Version(rules), nil
}

func (s *testService) Prune(_ context.Context, _ time.Time) (int64, error) { return 0, nil }

func compareStr(a, b string) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

type testTxm struct{}

func (t *testTxm) TxFn(ctx context.Context, f func(context.Context) error) error {
	if err := f(ctx); err != nil {
		return fmt.Errorf("transaction function: %w", err)
	}
	return nil
}

type ctxKey struct{}

type testSession struct{}

func (s *testSession) FromContext(ctx context.Context) *sessionModel.Session {
	if v, ok := ctx.Value(ctxKey{}).(*sessionModel.Session); ok {
		return v
	}
	return sessionModel.New(0)
}

type testRoot struct{ root *rootModel.Root }

func (r *testRoot) Get(_ context.Context) (*rootModel.Root, error) { return r.root, nil }

type testUsr struct{}

func (u *testUsr) Get(_ context.Context, id int64, _ bool) (*usrModel.Usr, bool, error) {
	return &usrModel.Usr{Id: id, Name: "Ivan"}, true, nil
}

type testNotifier struct{ n int }

func (n *testNotifier) NotifyBlocklist() { n.n++ }

type env struct {
	uc       *Usecase
	svc      *testService
	notifier *testNotifier
	now      time.Time
}

func newEnv() *env {
	e := &env{svc: &testService{}, notifier: &testNotifier{}, now: time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC)}
	root := &rootModel.Root{TrustedClients: []rootModel.RootTrustedClient{
		{Name: "office", Ip: "198.51.100.0/24"},
		{Name: "partner", Ip: "2001:db8:aa::/48"},
	}}
	e.uc = New(e.svc, &testTxm{}, &testSession{}, &testRoot{root: root}, &testUsr{}, e.notifier)
	e.uc.now = func() time.Time { return e.now }
	return e
}

func admin() context.Context {
	return context.WithValue(context.Background(), ctxKey{}, &sessionModel.Session{Id: 1, Admin: true})
}

func agent(keyId string) context.Context {
	return context.WithValue(context.Background(), ctxKey{}, &sessionModel.Session{Id: 1, Admin: true, ApiKeyId: keyId, ApiKeyName: "pulse"})
}

// errCode is the sentinel of a ruto error: ErrFull does not unwrap.
func errCode(err error) error {
	if full, ok := errors.AsType[errs.ErrFull](err); ok {
		return full.Err
	}
	if base, ok := errors.AsType[errs.Err](err); ok {
		return base
	}
	return err
}

func TestCreateGuards(t *testing.T) {
	cases := []struct {
		name     string
		agent    bool
		ip       string
		reason   string
		duration time.Duration
		wantErr  error // nil = allowed
		wantIp   string
	}{
		{name: "agent single address", agent: true, ip: "203.0.113.7", duration: time.Hour, wantIp: "203.0.113.7/32"},
		{name: "agent /24", agent: true, ip: "203.0.113.9/24", duration: time.Hour, wantIp: "203.0.113.0/24"},
		{name: "agent /23 too wide", agent: true, ip: "203.0.112.0/23", duration: time.Hour, wantErr: errs.InvalidRequest},
		{name: "agent ipv6 /48", agent: true, ip: "2001:db8:1::/48", duration: time.Hour, wantIp: "2001:db8:1::/48"},
		{name: "agent ipv6 /47 too wide", agent: true, ip: "2001:db8::/47", duration: time.Hour, wantErr: errs.InvalidRequest},
		{name: "admin /16", ip: "203.0.0.0/16", duration: time.Hour, wantIp: "203.0.0.0/16"},
		{name: "admin /15 too wide", ip: "203.0.0.0/15", duration: time.Hour, wantErr: errs.InvalidRequest},
		{name: "admin ipv6 /32", ip: "2001:db9::/32", duration: time.Hour, wantIp: "2001:db9::/32"},
		{name: "admin ipv6 /31 too wide", ip: "2001:db8::/31", duration: time.Hour, wantErr: errs.InvalidRequest},
		{name: "not an address", agent: true, ip: "evil.example.com", duration: time.Hour, wantErr: errs.InvalidRequest},
		{name: "reason required", agent: true, ip: "203.0.113.7", reason: " ", duration: time.Hour, wantErr: errs.InvalidRequest},

		{name: "agent private 10/8", agent: true, ip: "10.1.2.3", duration: time.Hour, wantErr: errs.NoPermission},
		{name: "agent private 192.168", agent: true, ip: "192.168.5.0/24", duration: time.Hour, wantErr: errs.NoPermission},
		{name: "agent private 172.16/12", agent: true, ip: "172.20.0.1", duration: time.Hour, wantErr: errs.NoPermission},
		{name: "agent loopback", agent: true, ip: "127.0.0.1", duration: time.Hour, wantErr: errs.NoPermission},
		{name: "agent cgnat", agent: true, ip: "100.64.1.1", duration: time.Hour, wantErr: errs.NoPermission},
		{name: "agent link-local", agent: true, ip: "169.254.1.1", duration: time.Hour, wantErr: errs.NoPermission},
		{name: "agent multicast", agent: true, ip: "224.0.0.1", duration: time.Hour, wantErr: errs.NoPermission},
		{name: "agent reserved", agent: true, ip: "250.1.1.1", duration: time.Hour, wantErr: errs.NoPermission},
		{name: "agent zero net", agent: true, ip: "0.1.2.3", duration: time.Hour, wantErr: errs.NoPermission},
		{name: "agent ipv6 loopback", agent: true, ip: "::1", duration: time.Hour, wantErr: errs.NoPermission},
		{name: "agent ipv6 unspecified", agent: true, ip: "::", duration: time.Hour, wantErr: errs.NoPermission},
		{name: "agent ipv6 ula", agent: true, ip: "fd00:1::5", duration: time.Hour, wantErr: errs.NoPermission},
		{name: "agent ipv6 link-local", agent: true, ip: "fe80::1", duration: time.Hour, wantErr: errs.NoPermission},
		{name: "agent ipv6 multicast", agent: true, ip: "ff02::1", duration: time.Hour, wantErr: errs.NoPermission},
		{name: "agent ipv4-mapped private", agent: true, ip: "::ffff:10.1.2.3", duration: time.Hour, wantErr: errs.NoPermission},
		{name: "agent ipv4-mapped private network", agent: true, ip: "::ffff:192.168.1.0/120", duration: time.Hour, wantErr: errs.NoPermission},
		{name: "agent ipv4-mapped prefix kept as ipv6", agent: true, ip: "::ffff:10.0.0.0/90", duration: time.Hour, wantErr: errs.NoPermission},
		{name: "agent ipv4-compatible", agent: true, ip: "::10.1.2.3", duration: time.Hour, wantErr: errs.NoPermission},
		{name: "agent ipv4-mapped public", agent: true, ip: "::ffff:203.0.113.7", duration: time.Hour, wantIp: "203.0.113.7/32"},
		{name: "admin may block private", ip: "10.1.2.0/24", duration: time.Hour, wantIp: "10.1.2.0/24"},

		{name: "inside trusted", agent: true, ip: "198.51.100.7", duration: time.Hour, wantErr: errs.NoPermission},
		{name: "contains trusted", ip: "198.51.0.0/16", duration: time.Hour, wantErr: errs.NoPermission},
		{name: "admin inside trusted", ip: "198.51.100.7", duration: time.Hour, wantErr: errs.NoPermission},
		{name: "trusted ipv6", agent: true, ip: "2001:db8:aa:1::5", duration: time.Hour, wantErr: errs.NoPermission},
		{name: "next to trusted", agent: true, ip: "198.51.101.7", duration: time.Hour, wantIp: "198.51.101.7/32"},

		{name: "agent without duration", agent: true, ip: "203.0.113.7", duration: 0, wantErr: errs.InvalidRequest},
		{name: "agent duration below 1m", agent: true, ip: "203.0.113.7", duration: 30 * time.Second, wantErr: errs.InvalidRequest},
		{name: "agent 1m", agent: true, ip: "203.0.113.7", duration: time.Minute, wantIp: "203.0.113.7/32"},
		{name: "agent 7d", agent: true, ip: "203.0.113.7", duration: 7 * 24 * time.Hour, wantIp: "203.0.113.7/32"},
		{name: "agent over 7d", agent: true, ip: "203.0.113.7", duration: 7*24*time.Hour + time.Second, wantErr: errs.InvalidRequest},
		{name: "admin without end", ip: "203.0.113.7", duration: 0, wantIp: "203.0.113.7/32"},
		{name: "admin negative", ip: "203.0.113.7", duration: -time.Hour, wantErr: errs.InvalidRequest},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv()
			ctx := admin()
			if tc.agent {
				ctx = agent("key1")
			}
			reason := lo.Ternary(tc.reason == "", "scan of /wp-admin", tc.reason)

			item, err := e.uc.Create(ctx, &CreateReq{Ip: tc.ip, Reason: reason, Duration: tc.duration})
			if tc.wantErr != nil {
				require.Error(t, err)
				assert.Equal(t, tc.wantErr, errCode(err), err.Error())
				assert.Empty(t, e.svc.items)
				assert.Zero(t, e.notifier.n)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantIp, item.Ip)
			assert.Equal(t, lo.Ternary(tc.agent, model.SourceAgent, model.SourceAdmin), item.Source)
			assert.Equal(t, lo.Ternary(tc.agent, "Ivan (key pulse)", "Ivan"), item.CreatedBy)
			if tc.duration == 0 {
				assert.Nil(t, item.ExpiresAt)
			} else {
				assert.Equal(t, e.now.Add(tc.duration), *item.ExpiresAt)
			}
			assert.Equal(t, 1, e.notifier.n)
		})
	}
}

func TestTrustedErrorNamesEntry(t *testing.T) {
	e := newEnv()
	_, err := e.uc.Create(agent("k"), &CreateReq{Ip: "198.51.100.7", Reason: "x", Duration: time.Hour})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"office"`)
	assert.Contains(t, err.Error(), "198.51.100.0/24")
}

func TestCreateNeedsAdmin(t *testing.T) {
	e := newEnv()

	_, err := e.uc.Create(context.Background(), &CreateReq{Ip: "203.0.113.7", Reason: "x", Duration: time.Hour})
	assert.Equal(t, errs.NotAuthorized, err)

	editor := context.WithValue(context.Background(), ctxKey{}, &sessionModel.Session{Id: 2, AllApps: true})
	_, err = e.uc.Create(editor, &CreateReq{Ip: "203.0.113.7", Reason: "x", Duration: time.Hour})
	assert.Equal(t, errs.NoPermission, err)

	_, err = e.uc.List(editor, false)
	assert.Equal(t, errs.NoPermission, err)
	_, err = e.uc.Remove(editor, &RemoveReq{Ip: "203.0.113.7"})
	assert.Equal(t, errs.NoPermission, err)
}

func TestProlong(t *testing.T) {
	cases := []struct {
		name       string
		firstAgent bool
		first      time.Duration
		nextAgent  bool
		next       time.Duration
		// expiry after the second call, from the first call; 0 = without end
		want       time.Duration
		wantSource string
	}{
		{name: "agent extends agent", firstAgent: true, first: time.Hour, nextAgent: true, next: 2 * time.Hour, want: 2*time.Hour + time.Minute, wantSource: model.SourceAgent},
		{name: "agent cannot shorten agent", firstAgent: true, first: 2 * time.Hour, nextAgent: true, next: time.Hour, want: 2 * time.Hour, wantSource: model.SourceAgent},
		{name: "admin makes it without end", firstAgent: true, first: time.Hour, next: 0, want: 0, wantSource: model.SourceAdmin},
		{name: "admin confirms an agent block", firstAgent: true, first: 2 * time.Hour, next: time.Hour, want: 2 * time.Hour, wantSource: model.SourceAdmin},
		{name: "admin extends admin", first: time.Hour, next: 48 * time.Hour, want: 48*time.Hour + time.Minute, wantSource: model.SourceAdmin},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv()
			ctxOf := func(isAgent bool) context.Context { return lo.Ternary(isAgent, agent("k"), admin()) }

			first, err := e.uc.Create(ctxOf(tc.firstAgent), &CreateReq{Ip: "203.0.113.7", Reason: "first", Duration: tc.first})
			require.NoError(t, err)

			e.now = e.now.Add(time.Minute)
			second, err := e.uc.Create(ctxOf(tc.nextAgent), &CreateReq{Ip: "203.0.113.7/32", Reason: "second", Duration: tc.next})
			require.NoError(t, err)

			require.Len(t, e.svc.items, 1, "a prolong keeps one row")
			assert.Equal(t, first.Id, second.Id)
			assert.Equal(t, "second", second.Reason)
			assert.Equal(t, first.CreatedBy, second.CreatedBy)
			assert.Equal(t, e.now, second.UpdatedAt)
			assert.Equal(t, tc.wantSource, second.Source)
			if tc.want == 0 {
				assert.Nil(t, second.ExpiresAt)
			} else {
				assert.Equal(t, first.CreatedAt.Add(tc.want), *second.ExpiresAt)
			}
			assert.Equal(t, 2, e.notifier.n)
		})
	}
}

// An agent never rewrites a person's record: the block already holds.
func TestAgentLeavesPersonBlockAsIs(t *testing.T) {
	for _, first := range []time.Duration{0, time.Hour, 48 * time.Hour} {
		e := newEnv()
		person, err := e.uc.Create(admin(), &CreateReq{Ip: "203.0.113.7", Reason: "person", Duration: first})
		require.NoError(t, err)

		e.now = e.now.Add(time.Minute)
		got, err := e.uc.Create(agent("k"), &CreateReq{Ip: "203.0.113.7", Reason: "agent", Duration: 7 * 24 * time.Hour})
		require.NoError(t, err)

		assert.Equal(t, person, got, "returned as is")
		stored, _, _ := e.svc.Get(context.Background(), person.Id, true)
		assert.Equal(t, person, stored, "nothing rewritten: reason, expiry, updated_at")
		assert.Equal(t, 1, e.notifier.n, "no change, no notification")
	}
}

// The log line tells what happened, not what was asked.
func TestCreateLogsOutcome(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	e := newEnv()
	_, err := e.uc.Create(admin(), &CreateReq{Ip: "203.0.113.7", Reason: "person"})
	require.NoError(t, err)
	buf.Reset()

	_, err = e.uc.Create(agent("k"), &CreateReq{Ip: "203.0.113.7", Reason: "agent", Duration: time.Hour})
	require.NoError(t, err)

	line := map[string]any{}
	require.NoError(t, json.Unmarshal(buf.Bytes(), &line))
	assert.Equal(t, "blocked client: "+outcomeUnchanged, line["msg"])
	assert.Equal(t, model.SourceAdmin, line["source"])
	assert.Nil(t, line["expires_at"], "the stored expiry (none), not the requested hour")
	assert.Equal(t, "person", line["reason"])
	assert.Equal(t, model.SourceAgent, line["caller_source"])
}

// An agent cannot carry its block past created_at + 7 days.
func TestAgentProlongLimit(t *testing.T) {
	e := newEnv()
	first, err := e.uc.Create(agent("k"), &CreateReq{Ip: "203.0.113.7", Reason: "x", Duration: 7 * 24 * time.Hour})
	require.NoError(t, err)

	e.now = e.now.Add(6 * 24 * time.Hour)
	second, err := e.uc.Create(agent("k"), &CreateReq{Ip: "203.0.113.7", Reason: "still", Duration: 7 * 24 * time.Hour})
	require.NoError(t, err)
	assert.Equal(t, first.CreatedAt.Add(AgentMaxBlockAge), *second.ExpiresAt)

	e.now = e.now.Add(12 * time.Hour)
	third, err := e.uc.Create(agent("k"), &CreateReq{Ip: "203.0.113.7", Reason: "still", Duration: time.Hour})
	require.NoError(t, err)
	assert.Equal(t, first.CreatedAt.Add(AgentMaxBlockAge), *third.ExpiresAt)

	// after it expired, a new block is a new row with its own 7 days
	e.now = e.now.Add(24 * time.Hour)
	fresh, err := e.uc.Create(agent("k"), &CreateReq{Ip: "203.0.113.7", Reason: "back", Duration: 7 * 24 * time.Hour})
	require.NoError(t, err)
	assert.NotEqual(t, first.Id, fresh.Id)
	assert.Equal(t, e.now.Add(7*24*time.Hour), *fresh.ExpiresAt)

	// a person is not limited
	person, err := e.uc.Create(admin(), &CreateReq{Ip: "203.0.113.7", Reason: "person", Duration: 30 * 24 * time.Hour})
	require.NoError(t, err)
	assert.Equal(t, e.now.Add(30*24*time.Hour), *person.ExpiresAt)
}

func TestCreateAfterExpiry(t *testing.T) {
	e := newEnv()
	first, err := e.uc.Create(agent("k"), &CreateReq{Ip: "203.0.113.7", Reason: "first", Duration: time.Hour})
	require.NoError(t, err)

	e.now = e.now.Add(2 * time.Hour)
	second, err := e.uc.Create(agent("k"), &CreateReq{Ip: "203.0.113.7", Reason: "again", Duration: time.Hour})
	require.NoError(t, err)

	assert.NotEqual(t, first.Id, second.Id)
	require.Len(t, e.svc.items, 2)
	old, _, _ := e.svc.Get(context.Background(), first.Id, true)
	require.NotNil(t, old.RemovedAt, "the expired block is closed at its expiry")
	assert.Equal(t, *first.ExpiresAt, *old.RemovedAt)
}

// The hourly budget of new agent blocks is shared by all keys and counted
// from the stored rows.
func TestAgentHourlyBudget(t *testing.T) {
	e := newEnv()
	for i := range AgentNewBlocksPerHour {
		_, err := e.uc.Create(agent(fmt.Sprintf("k%d", i%3)), &CreateReq{Ip: fmt.Sprintf("203.0.113.%d", i), Reason: "x", Duration: time.Hour})
		require.NoError(t, err)
	}

	_, err := e.uc.Create(agent("another-key"), &CreateReq{Ip: "203.0.113.200", Reason: "x", Duration: time.Hour})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "all keys together")

	// prolongs do not count, a person is not affected
	_, err = e.uc.Create(agent("k0"), &CreateReq{Ip: "203.0.113.0", Reason: "x", Duration: 2 * time.Hour})
	require.NoError(t, err)
	_, err = e.uc.Create(admin(), &CreateReq{Ip: "203.0.113.202", Reason: "x", Duration: time.Hour})
	require.NoError(t, err)

	// a fresh core (new usecase over the same rows) sees the same budget
	e.uc = New(e.svc, &testTxm{}, &testSession{}, &testRoot{root: &rootModel.Root{}}, &testUsr{}, e.notifier)
	e.uc.now = func() time.Time { return e.now }
	_, err = e.uc.Create(agent("k1"), &CreateReq{Ip: "203.0.113.200", Reason: "x", Duration: time.Hour})
	require.Error(t, err, "the budget survives a restart")

	e.now = e.now.Add(time.Hour + time.Second)
	_, err = e.uc.Create(agent("k1"), &CreateReq{Ip: "203.0.113.200", Reason: "x", Duration: 3 * time.Hour})
	require.NoError(t, err, "the window rolls")
}

func addActive(e *env, n int, source string) {
	for i := range n {
		e.svc.items = append(e.svc.items, &model.BlockedClient{
			Id: source + strconv.Itoa(i), Ip: fmt.Sprintf("10.%d.%d.%d/32", i>>16, (i>>8)&255, i&255), Source: source,
			CreatedAt: e.now.Add(-2 * time.Hour),
		})
	}
}

// Agents have their own ceiling and can never use up the room of people.
func TestCaps(t *testing.T) {
	t.Run("agent ceiling", func(t *testing.T) {
		e := newEnv()
		addActive(e, MaxAgentActive, model.SourceAgent)

		_, err := e.uc.Create(agent("k"), &CreateReq{Ip: "203.0.113.7", Reason: "x", Duration: time.Hour})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "most allowed for agents")

		_, err = e.uc.Create(admin(), &CreateReq{Ip: "203.0.113.7", Reason: "x", Duration: time.Hour})
		require.NoError(t, err, "a person still blocks")
	})

	t.Run("people ceiling ignores agent blocks", func(t *testing.T) {
		e := newEnv()
		addActive(e, MaxActive-1, model.SourceAdmin)
		for i := range MaxAgentActive {
			e.svc.items = append(e.svc.items, &model.BlockedClient{Id: "a" + strconv.Itoa(i), Ip: fmt.Sprintf("203.0.%d.%d/32", 100+i/256, i%256), Source: model.SourceAgent})
		}

		_, err := e.uc.Create(admin(), &CreateReq{Ip: "198.18.0.1", Reason: "x", Duration: time.Hour})
		require.NoError(t, err)

		_, err = e.uc.Create(admin(), &CreateReq{Ip: "198.18.0.2", Reason: "x", Duration: time.Hour})
		require.Error(t, err)
		assert.Equal(t, errs.InvalidRequest, errCode(err))

		// prolonging an existing block is not a new one
		_, err = e.uc.Create(admin(), &CreateReq{Ip: "10.0.0.5", Reason: "x", Duration: time.Hour})
		require.NoError(t, err)
	})
}

// After a person lifts a block, agents keep off overlapping networks for 7 days.
func TestAgentCooldownAfterPersonLift(t *testing.T) {
	e := newEnv()
	block, err := e.uc.Create(agent("k"), &CreateReq{Ip: "203.0.113.7", Reason: "x", Duration: time.Hour})
	require.NoError(t, err)
	_, err = e.uc.Remove(admin(), &RemoveReq{Id: block.Id, Reason: "our partner"})
	require.NoError(t, err)

	e.now = e.now.Add(6 * 24 * time.Hour)
	for _, ip := range []string{"203.0.113.7", "203.0.113.0/24"} {
		_, err = e.uc.Create(agent("other"), &CreateReq{Ip: ip, Reason: "x", Duration: time.Hour})
		require.Error(t, err, ip)
		assert.Equal(t, errs.NoPermission, errCode(err))
		assert.Contains(t, err.Error(), "our partner")
		assert.Contains(t, err.Error(), "2026-11-24T12:00:00Z")
	}
	_, err = e.uc.Create(agent("other"), &CreateReq{Ip: "203.0.113.8", Reason: "x", Duration: time.Hour})
	require.NoError(t, err, "a network that does not overlap")
	_, err = e.uc.Create(admin(), &CreateReq{Ip: "203.0.113.7", Reason: "x", Duration: time.Hour})
	require.NoError(t, err, "a person may")
	_, err = e.uc.Remove(admin(), &RemoveReq{Ip: "203.0.113.7"})
	require.NoError(t, err)

	// an agent lifting its own block does not start a cooldown
	own, err := e.uc.Create(agent("k"), &CreateReq{Ip: "203.0.113.9", Reason: "x", Duration: time.Hour})
	require.NoError(t, err)
	_, err = e.uc.Remove(agent("k"), &RemoveReq{Id: own.Id, Reason: "mistake"})
	require.NoError(t, err)
	_, err = e.uc.Create(agent("k"), &CreateReq{Ip: "203.0.113.9", Reason: "x", Duration: time.Hour})
	require.NoError(t, err)

	e.now = e.now.Add(7 * 24 * time.Hour)
	_, err = e.uc.Create(agent("other"), &CreateReq{Ip: "203.0.113.7", Reason: "x", Duration: time.Hour})
	require.NoError(t, err, "the cooldown is over")
}

func TestRemoveAgentBlocks(t *testing.T) {
	e := newEnv()
	for i, key := range []string{"k1", "k1", "k2"} {
		_, err := e.uc.Create(agent(key), &CreateReq{Ip: fmt.Sprintf("203.0.113.%d", i), Reason: "x", Duration: time.Hour})
		require.NoError(t, err)
	}
	person, err := e.uc.Create(admin(), &CreateReq{Ip: "203.0.113.50", Reason: "x"})
	require.NoError(t, err)
	assert.Equal(t, "k1", e.svc.items[0].ApiKeyId)
	assert.Empty(t, person.ApiKeyId)
	e.notifier.n = 0

	_, err = e.uc.RemoveAgentBlocks(agent("k1"), "", "x")
	assert.Equal(t, errs.NoPermission, err, "never for a key")
	_, err = e.uc.RemoveAgentBlocks(admin(), "", " ")
	assert.Equal(t, errs.InvalidRequest, errCode(err))

	n, err := e.uc.RemoveAgentBlocks(admin(), "k1", "k1 misbehaves")
	require.NoError(t, err)
	assert.EqualValues(t, 2, n)
	assert.Equal(t, 1, e.notifier.n, "one notification")
	assert.Equal(t, model.SourceAdmin, e.svc.items[0].RemovedSource)
	assert.Equal(t, "k1 misbehaves", e.svc.items[0].RemoveReason)
	assert.Nil(t, e.svc.items[2].RemovedAt)

	n, err = e.uc.RemoveAgentBlocks(admin(), "", "all off")
	require.NoError(t, err)
	assert.EqualValues(t, 1, n)
	assert.Nil(t, e.svc.items[3].RemovedAt, "a person's block stays")

	n, err = e.uc.RemoveAgentBlocks(admin(), "", "again")
	require.NoError(t, err)
	assert.Zero(t, n)
	assert.Equal(t, 2, e.notifier.n, "nothing lifted, no notification")
}

func TestRemoveKeyBlocks(t *testing.T) {
	e := newEnv()
	_, err := e.uc.Create(agent("k1"), &CreateReq{Ip: "203.0.113.1", Reason: "x", Duration: time.Hour})
	require.NoError(t, err)
	_, err = e.uc.Create(agent("k2"), &CreateReq{Ip: "203.0.113.2", Reason: "x", Duration: time.Hour})
	require.NoError(t, err)

	n, err := e.uc.RemoveKeyBlocks(admin(), "k1", "pulse")
	require.NoError(t, err)
	assert.EqualValues(t, 1, n)
	assert.Equal(t, "api key pulse deleted", e.svc.items[0].RemoveReason)
	assert.Equal(t, model.RemovedSourceKeyDeleted, e.svc.items[0].RemovedSource)
	assert.Nil(t, e.svc.items[1].RemovedAt)

	// a deleted (maybe rotated) key is not a person's verdict: no cooldown
	_, err = e.uc.Create(agent("k3"), &CreateReq{Ip: "203.0.113.1", Reason: "x", Duration: time.Hour})
	require.NoError(t, err)
}

func TestRemove(t *testing.T) {
	e := newEnv()
	byAgent, err := e.uc.Create(agent("k"), &CreateReq{Ip: "203.0.113.7", Reason: "x", Duration: time.Hour})
	require.NoError(t, err)
	byAdmin, err := e.uc.Create(admin(), &CreateReq{Ip: "203.0.113.8", Reason: "x"})
	require.NoError(t, err)

	t.Run("agent cannot lift a person's block", func(t *testing.T) {
		_, err := e.uc.Remove(agent("k"), &RemoveReq{Id: byAdmin.Id, Reason: "false positive"})
		require.Error(t, err)
		assert.Equal(t, errs.NoPermission, errCode(err))
	})
	t.Run("agent needs a reason", func(t *testing.T) {
		_, err := e.uc.Remove(agent("k"), &RemoveReq{Id: byAgent.Id})
		assert.Equal(t, errs.InvalidRequest, errCode(err))
	})
	t.Run("unknown network", func(t *testing.T) {
		_, err := e.uc.Remove(agent("k"), &RemoveReq{Ip: "203.0.113.9", Reason: "x"})
		assert.Equal(t, errs.ObjectNotFound, errCode(err))
	})
	t.Run("agent lifts an agent block by ip", func(t *testing.T) {
		item, err := e.uc.Remove(agent("other-key"), &RemoveReq{Ip: "::ffff:203.0.113.7", Reason: "false positive"})
		require.NoError(t, err)
		assert.Equal(t, byAgent.Id, item.Id)
		assert.NotNil(t, item.RemovedAt)
		assert.Equal(t, "Ivan (key pulse)", item.RemovedBy)
		assert.Equal(t, "false positive", item.RemoveReason)
	})
	t.Run("already lifted", func(t *testing.T) {
		_, err := e.uc.Remove(admin(), &RemoveReq{Id: byAgent.Id})
		assert.Equal(t, errs.InvalidRequest, errCode(err))
	})
	t.Run("admin lifts anything without a reason", func(t *testing.T) {
		item, err := e.uc.Remove(admin(), &RemoveReq{Id: byAdmin.Id})
		require.NoError(t, err)
		assert.Equal(t, "Ivan", item.RemovedBy)
	})

	assert.Len(t, e.svc.items, 2, "lifted rows stay as history")
	assert.Equal(t, 4, e.notifier.n)

	items, err := e.uc.List(admin(), false)
	require.NoError(t, err)
	assert.Empty(t, items)
	items, err = e.uc.List(admin(), true)
	require.NoError(t, err)
	assert.Len(t, items, 2)
}

func TestVersionFollowsActiveSet(t *testing.T) {
	e := newEnv()
	ctx := context.Background()

	empty, err := e.uc.GetVersion(ctx)
	require.NoError(t, err)

	_, err = e.uc.Create(agent("k"), &CreateReq{Ip: "203.0.113.7", Reason: "x", Duration: time.Hour})
	require.NoError(t, err)
	one, err := e.uc.GetVersion(ctx)
	require.NoError(t, err)
	assert.NotEqual(t, empty, one)

	rules, version, err := e.uc.GetActive(ctx)
	require.NoError(t, err)
	assert.Equal(t, one, version)
	require.Len(t, rules, 1)
	assert.Equal(t, "203.0.113.7/32", rules[0].Ip)

	e.now = e.now.Add(time.Minute)
	_, err = e.uc.Create(agent("k"), &CreateReq{Ip: "203.0.113.7", Reason: "x", Duration: 2 * time.Hour})
	require.NoError(t, err)
	prolonged, _ := e.uc.GetVersion(ctx)
	assert.NotEqual(t, one, prolonged, "a new expiry is a new version")

	e.now = e.now.Add(3 * time.Hour)
	expired, _ := e.uc.GetVersion(ctx)
	assert.Equal(t, empty, expired, "an expired block leaves the set without a write")
}
