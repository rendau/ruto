package changerequest

import (
	"context"
	"encoding/json"
	"maps"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appModel "github.com/rendau/ruto/internal/domain/app/model"
	"github.com/rendau/ruto/internal/domain/changerequest/model"
	endpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
	seenPathModel "github.com/rendau/ruto/internal/domain/seenpath/model"
	sessionModel "github.com/rendau/ruto/internal/domain/session/model"
	usrModel "github.com/rendau/ruto/internal/domain/usr/model"
	"github.com/rendau/ruto/internal/errs"
)

// memStore keeps the configuration a change request touches in memory; its
// TxFn rolls the state back when f fails, like the database transaction.
type memStore struct {
	apps      []*appModel.App
	endpoints []*endpointModel.Endpoint
	seen      []*seenPathModel.SeenPath
	requests  map[int64]*model.ChangeRequest
}

func (s *memStore) TxFn(ctx context.Context, f func(context.Context) error) error {
	endpoints, seen, requests := slices.Clone(s.endpoints), slices.Clone(s.seen), maps.Clone(s.requests)
	if err := f(ctx); err != nil {
		s.endpoints, s.seen, s.requests = endpoints, seen, requests
		return err
	}
	return nil
}

// ServiceI

func (s *memStore) List(_ context.Context, pars *model.ListReq) ([]*model.ChangeRequest, int64, error) {
	items := lo.Filter(lo.Values(s.requests), func(v *model.ChangeRequest, _ int) bool {
		return pars.Status == nil || v.Status == *pars.Status
	})
	return items, int64(len(items)), nil
}

func (s *memStore) Get(_ context.Context, id int64, _ bool) (*model.ChangeRequest, bool, error) {
	item, ok := s.requests[id]
	if !ok {
		return nil, false, errs.ObjectNotFound
	}
	return new(*item), true, nil
}

func (s *memStore) GetLocked(ctx context.Context, id int64) (*model.ChangeRequest, error) {
	item, _, err := s.Get(ctx, id, true)
	return item, err
}

func (s *memStore) Create(_ context.Context, obj *model.ChangeRequest) (int64, error) {
	obj.Id = int64(len(s.requests) + 1)
	obj.Status = model.StatusPending
	obj.CreatedAt = time.Now()
	s.requests[obj.Id] = obj
	return obj.Id, nil
}

func (s *memStore) Update(_ context.Context, id int64, obj *model.Edit) error {
	item := new(*s.requests[id])
	if obj.Status != nil {
		item.Status = *obj.Status
	}
	s.requests[id] = item
	return nil
}

func (s *memStore) Cleanup(_ context.Context, _, _ time.Duration) (int64, int64, error) {
	return 0, 0, nil
}

type memApps struct{ s *memStore }

func (r *memApps) List(_ context.Context, pars *appModel.ListReq) ([]*appModel.App, int64, error) {
	items := lo.Filter(r.s.apps, func(v *appModel.App, _ int) bool {
		return pars.NameEqCI == nil || v.Name == *pars.NameEqCI
	})
	return items, int64(len(items)), nil
}

func (r *memApps) Get(_ context.Context, id string, _ bool) (*appModel.App, bool, error) {
	item, ok := lo.Find(r.s.apps, func(v *appModel.App) bool { return v.Id == id })
	if !ok {
		return nil, false, nil
	}
	return new(*item), true, nil
}

type memEndpoints struct{ s *memStore }

func (r *memEndpoints) List(_ context.Context, _ *endpointModel.ListReq) ([]*endpointModel.Endpoint, int64, error) {
	return r.s.endpoints, int64(len(r.s.endpoints)), nil
}

func (r *memEndpoints) Get(_ context.Context, id string, _ bool) (*endpointModel.Endpoint, bool, error) {
	item, ok := lo.Find(r.s.endpoints, func(v *endpointModel.Endpoint) bool { return v.Id == id })
	return item, ok, nil
}

func (r *memEndpoints) Create(_ context.Context, obj *endpointModel.Endpoint) (string, error) {
	obj.Id = "ep-" + strconv.Itoa(len(r.s.endpoints)+1)
	r.s.endpoints = append(r.s.endpoints, obj)
	return obj.Id, nil
}

func (r *memEndpoints) Update(_ context.Context, _ string, _ *endpointModel.Endpoint) error {
	return nil
}

func (r *memEndpoints) Delete(_ context.Context, id string) error {
	r.s.endpoints = lo.Reject(r.s.endpoints, func(v *endpointModel.Endpoint, _ int) bool { return v.Id == id })
	return nil
}

type memSeenPaths struct{ s *memStore }

func (r *memSeenPaths) ListByPaths(_ context.Context, appId string, keys []*seenPathModel.PathKey) ([]*seenPathModel.SeenPath, error) {
	return lo.Filter(r.s.seen, func(v *seenPathModel.SeenPath, _ int) bool {
		return v.AppId == appId && lo.ContainsBy(keys, func(k *seenPathModel.PathKey) bool {
			return k.Method == v.Method && k.Path == v.Path
		})
	}), nil
}

func (r *memSeenPaths) Delete(_ context.Context, appId string, keys []*seenPathModel.SeenPathKey) error {
	r.s.seen = lo.Reject(r.s.seen, func(v *seenPathModel.SeenPath, _ int) bool {
		return v.AppId == appId && lo.ContainsBy(keys, func(k *seenPathModel.SeenPathKey) bool {
			return k.EndpointId == v.EndpointId && k.Method == v.Method && k.Path == v.Path
		})
	})
	return nil
}

type noSnapshot struct{}

func (noSnapshot) Validate(_ context.Context) error { return nil }
func (noSnapshot) Refresh(_ context.Context) error  { return nil }

type noUsr struct{}

func (noUsr) Get(_ context.Context, _ int64, _ bool) (*usrModel.Usr, bool, error) {
	return nil, false, nil
}

func newSeenPathsTestUsecase() (*Usecase, *memStore, *testSessionService) {
	store := &memStore{
		apps: []*appModel.App{{Id: "app-1", Name: "orders", AllowWildcard: true}},
		endpoints: []*endpointModel.Endpoint{{
			Id: "ep-wild", AppId: "app-1", Type: endpointModel.TypeHTTP, TrackPaths: true,
			Http: endpointModel.Http{Method: "GET", Path: "*"},
		}},
		seen: []*seenPathModel.SeenPath{
			{AppId: "app-1", EndpointId: "ep-wild", Method: "GET", Path: "users/{id}", Hits: 42, Sample: "users/17"},
			{AppId: "app-1", EndpointId: "ep-wild", Method: "GET", Path: "orders", Hits: 5, Sample: "orders"},
		},
		requests: map[int64]*model.ChangeRequest{},
	}
	session := &testSessionService{}
	endpoints := &memEndpoints{s: store}
	seen := &memSeenPaths{s: store}

	uc := New(
		Config{PendingTTL: time.Hour, MaxPendingPerKey: 10},
		store, store, session,
		nil, nil, endpoints, seen, noSnapshot{},
		nil, &memApps{s: store}, endpoints, seen, noUsr{},
	)

	return uc, store, session
}

// assertErrBase checks the sentinel code: ErrFull does not unwrap to it.
func assertErrBase(t *testing.T, expected, err error, msgAndArgs ...any) {
	t.Helper()
	base, _ := errsBase(err)
	assert.Equal(t, expected, base, msgAndArgs...)
}

var (
	agentSession = &sessionModel.Session{Id: 1, Admin: true, ApiKeyId: "key-1"}
	adminSession = &sessionModel.Session{Id: 2, Admin: true}
)

func seenPathsOp(app string, rows ...model.SeenPathRow) *ProposeOp {
	data, _ := json.Marshal(model.SeenPathsData{App: app, Rows: rows})
	return &ProposeOp{Op: model.Op{Op: model.OpDeleteSeenPaths, Data: data}}
}

func TestDeleteSeenPaths_WithCreateEndpoint(t *testing.T) {
	uc, store, session := newSeenPathsTestUsecase()
	ctx := context.Background()

	session.session = agentSession
	id, err := uc.Propose(ctx, &ProposeReq{
		Title: "Exact endpoint for users/{id}",
		Ops: []*ProposeOp{
			{Op: model.Op{
				Op:   model.OpCreateEndpoint,
				App:  "app-1",
				Data: json.RawMessage(`{"type":"http","http":{"method":"GET","path":"users/{id}"}}`),
			}},
			// written as an agent may: by app name, lower-case method, leading slash
			seenPathsOp("orders", model.SeenPathRow{Method: "get", Path: "/users/{id}"}),
		},
	})
	require.NoError(t, err)

	// the proposal check runs in a rolled back transaction
	require.Len(t, store.endpoints, 1)
	require.Len(t, store.seen, 2)

	var data model.SeenPathsData
	require.NoError(t, json.Unmarshal(store.requests[id].Plan.Ops[1].Data, &data))
	assert.Equal(t, model.SeenPathsData{App: "app-1", Rows: []model.SeenPathRow{{Method: "GET", Path: "users/{id}"}}}, data)

	detail, err := uc.Get(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, "orders", detail.Ops[1].AppName)
	assert.Equal(t, []*SeenPathView{{Method: "GET", Path: "users/{id}", Hits: 42, Sample: "users/17"}}, detail.Ops[1].SeenPaths)
	assert.Empty(t, detail.Ops[1].Notes)

	session.session = adminSession
	require.NoError(t, uc.Apply(ctx, id, false))

	assert.Equal(t, model.StatusApplied, store.requests[id].Status)
	assert.True(t, lo.ContainsBy(store.endpoints, func(v *endpointModel.Endpoint) bool {
		return v.AppId == "app-1" && v.Http.Path == "users/{id}"
	}), "the endpoint is created")
	assert.Equal(t, []string{"orders"}, lo.Map(store.seen, func(v *seenPathModel.SeenPath, _ int) string { return v.Path }),
		"only the requested row leaves the report")
}

func TestDeleteSeenPaths_RowNotInReport(t *testing.T) {
	uc, store, session := newSeenPathsTestUsecase()
	session.session = agentSession

	_, err := uc.Propose(context.Background(), &ProposeReq{
		Title: "Clean the report",
		Ops: []*ProposeOp{seenPathsOp("app-1",
			model.SeenPathRow{Method: "GET", Path: "users/{id}"},
			model.SeenPathRow{Method: "POST", Path: "users/{id}"},
		)},
	})

	require.Error(t, err)
	assertErrBase(t, errs.InvalidRequest, err)
	assert.Contains(t, errDesc(err), "POST /users/{id}")
	assert.Empty(t, store.requests, "no request is stored")
	assert.Len(t, store.seen, 2)
}

func TestDeleteSeenPaths_Shape(t *testing.T) {
	cases := map[string]string{
		"no app":       `{"rows":[{"method":"GET","path":"x"}]}`,
		"ref app":      `{"app":"$a","rows":[{"method":"GET","path":"x"}]}`,
		"no rows":      `{"app":"orders","rows":[]}`,
		"no method":    `{"app":"orders","rows":[{"path":"x"}]}`,
		"not a object": `[1]`,
	}
	for name, raw := range cases {
		_, err := parseSeenPaths(&model.Op{Op: model.OpDeleteSeenPaths, Data: json.RawMessage(raw)})
		assertErrBase(t, errs.InvalidRequest, err, name)
	}

	data, err := parseSeenPaths(&model.Op{Op: model.OpDeleteSeenPaths, Data: json.RawMessage(
		`{"app":" orders ","rows":[{"method":"get","path":"/a"},{"method":"GET","path":"a"}]}`)})
	require.NoError(t, err)
	assert.Equal(t, &model.SeenPathsData{App: "orders", Rows: []model.SeenPathRow{{Method: "GET", Path: "a"}}}, data)
}
