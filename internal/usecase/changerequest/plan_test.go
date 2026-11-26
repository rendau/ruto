package changerequest

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rendau/ruto/internal/domain/changerequest/model"
	endpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
	"github.com/rendau/ruto/internal/errs"
)

func op(kind string, f func(o *model.Op)) *model.Op {
	result := &model.Op{Op: kind}
	if f != nil {
		f(result)
	}
	return result
}

func withData(o *model.Op) { o.Data = json.RawMessage(`{"name":"x"}`) }

func TestCheckShape(t *testing.T) {
	valid := []*model.Op{
		op(model.OpCreateApp, func(o *model.Op) { o.Ref = "$a"; withData(o) }),
		op(model.OpCreateEndpoint, func(o *model.Op) { o.App = "$a"; withData(o) }),
		op(model.OpUpdateEndpoint, func(o *model.Op) { o.Id = "e1"; withData(o) }),
		op(model.OpDeleteApp, func(o *model.Op) { o.Id = "a2" }),
		op(model.OpUpdateRoot, withData),
		op(model.OpDeleteSeenPaths, withData),
	}
	require.NoError(t, checkShape(valid))

	cases := map[string][]*model.Op{
		"empty":            {},
		"unknown op":       {op("drop_db", withData)},
		"ref without $":    {op(model.OpCreateApp, func(o *model.Op) { o.Ref = "a"; withData(o) })},
		"duplicate ref":    {op(model.OpCreateApp, func(o *model.Op) { o.Ref = "$a"; withData(o) }), op(model.OpCreateApp, func(o *model.Op) { o.Ref = "$a"; withData(o) })},
		"ref used early":   {op(model.OpCreateEndpoint, func(o *model.Op) { o.App = "$a"; withData(o) }), op(model.OpCreateApp, func(o *model.Op) { o.Ref = "$a"; withData(o) })},
		"no app":           {op(model.OpCreateEndpoint, withData)},
		"update no id":     {op(model.OpUpdateApp, withData)},
		"update ref id":    {op(model.OpUpdateApp, func(o *model.Op) { o.Id = "$a"; withData(o) })},
		"update no data":   {op(model.OpUpdateApp, func(o *model.Op) { o.Id = "a" })},
		"delete with data": {op(model.OpDeleteEndpoint, func(o *model.Op) { o.Id = "e"; withData(o) })},
		"seen no data":     {op(model.OpDeleteSeenPaths, nil)},
	}
	for name, ops := range cases {
		assert.Error(t, checkShape(ops), name)
	}
}

func TestCollectRefs(t *testing.T) {
	ops := []*model.Op{
		op(model.OpCreateApp, func(o *model.Op) {
			o.Data = json.RawMessage(`{"variables":{
				"A":{"secret_ref":"app:x1/variables/T"},
				"B":{"generate":{"length":8},"name":"g"},
				"C":{"reuse":"g"},
				"D":"plain"}}`)
		}),
	}
	refs, err := collectRefs(ops)
	require.NoError(t, err)
	assert.Contains(t, refs, "app:x1")
	assert.Len(t, refs, 1)

	ops[0].Data = json.RawMessage(`{"variables":{"C":{"reuse":"missing"}}}`)
	_, err = collectRefs(ops)
	assert.Error(t, err)

	ops[0].Data = json.RawMessage(`{"variables":{"C":{"secret_ref":"app:x1/name"}}}`)
	_, err = collectRefs(ops)
	assert.Error(t, err, "a ref must point to a secret position")
}

func TestComparePaths(t *testing.T) {
	assert.Negative(t, comparePaths("auth/methods/2", "auth/methods/10"))
	assert.Negative(t, comparePaths("auth", "backend"))
	assert.Negative(t, comparePaths("a", "a/b"))
	assert.Zero(t, comparePaths("a/1", "a/1"))
}

func TestFlatten(t *testing.T) {
	got := flatten(map[string]any{
		"name":  "x",
		"tags":  []any{},
		"auth":  map[string]any{"methods": []any{map[string]any{"jwt": map[string]any{"roles": []any{"a"}}}}},
		"count": 2.0,
	})
	assert.Equal(t, map[string]string{
		"name":                       "x",
		"tags":                       "[]",
		"auth/methods/0/jwt/roles/0": "a",
		"count":                      "2",
	}, got)
}

type stubEndpointUsecase struct {
	created int
	updated int
}

func (s *stubEndpointUsecase) Create(_ context.Context, _ *endpointModel.Endpoint) (string, error) {
	s.created++
	return "ep-new", nil
}

func (s *stubEndpointUsecase) Update(_ context.Context, _ string, _ *endpointModel.Endpoint) error {
	s.updated++
	return nil
}

func (s *stubEndpointUsecase) Delete(_ context.Context, _ string) error { return nil }

type stubEndpointService struct {
	items map[string]*endpointModel.Endpoint
}

func (s *stubEndpointService) List(_ context.Context, _ *endpointModel.ListReq) ([]*endpointModel.Endpoint, int64, error) {
	return nil, 0, nil
}

func (s *stubEndpointService) Get(_ context.Context, id string, _ bool) (*endpointModel.Endpoint, bool, error) {
	ep, ok := s.items[id]
	if !ok {
		return nil, false, errs.ObjectNotFound
	}
	return ep, true, nil
}

func TestApplyOp_TransformScriptsForbidden(t *testing.T) {
	ctx := context.Background()
	resolveApp := func(v string) string { return v }

	t.Run("create_endpoint rejects request or response script", func(t *testing.T) {
		for name, transform := range map[string]map[string]any{
			"request":  {"request": "throw new Error(JSON.stringify(req.vars))"},
			"response": {"response": "return { body: res.vars }"},
		} {
			t.Run(name, func(t *testing.T) {
				epUc := &stubEndpointUsecase{}
				uc := &Usecase{endpointUc: epUc}
				res := &model.Result{Created: map[string]string{}}
				err := uc.applyOp(ctx, &model.Op{Op: model.OpCreateEndpoint, App: "app-1"}, map[string]any{
					"http":      map[string]any{"method": "GET", "path": "test"},
					"transform": transform,
				}, resolveApp, res, 0)
				base, ok := errsBase(err)
				require.True(t, ok)
				require.Equal(t, errs.InvalidRequest, base)
				assert.Contains(t, errDesc(err), "transform")
				assert.Zero(t, epUc.created)
			})
		}
	})

	t.Run("create_endpoint allows empty or whitespace transform", func(t *testing.T) {
		epUc := &stubEndpointUsecase{}
		uc := &Usecase{endpointUc: epUc}
		res := &model.Result{Created: map[string]string{}}
		err := uc.applyOp(ctx, &model.Op{Op: model.OpCreateEndpoint, App: "app-1"}, map[string]any{
			"http":      map[string]any{"method": "GET", "path": "test"},
			"transform": map[string]any{"request": "  ", "max_workers": 4},
		}, resolveApp, res, 0)
		require.NoError(t, err)
		assert.Equal(t, 1, epUc.created)
	})

	t.Run("update_endpoint rejects adding, changing or clearing scripts", func(t *testing.T) {
		cases := []struct {
			name      string
			current   endpointModel.Transform
			proposed  map[string]any
		}{
			{
				name:     "add request script",
				current:  endpointModel.Transform{},
				proposed: map[string]any{"request": "throw new Error(req.vars.SECRET)"},
			},
			{
				name:     "add response script",
				current:  endpointModel.Transform{},
				proposed: map[string]any{"response": "return { body: res.vars }"},
			},
			{
				name:     "modify existing request script",
				current:  endpointModel.Transform{Request: "return { body: req.body };"},
				proposed: map[string]any{"request": "return { response: { status: 200, body: req.vars } };"},
			},
			{
				name:     "clear existing response script",
				current:  endpointModel.Transform{Response: "return { body: res.body };"},
				proposed: map[string]any{"response": ""},
			},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				epUc := &stubEndpointUsecase{}
				epSvc := &stubEndpointService{
					items: map[string]*endpointModel.Endpoint{
						"ep-1": {Id: "ep-1", AppId: "app-1", Transform: tc.current},
					},
				}
				uc := &Usecase{endpointUc: epUc, endpointSvc: epSvc}
				res := &model.Result{Created: map[string]string{}}
				err := uc.applyOp(ctx, &model.Op{Op: model.OpUpdateEndpoint, Id: "ep-1"}, map[string]any{
					"active":    true,
					"transform": tc.proposed,
				}, resolveApp, res, 0)
				base, ok := errsBase(err)
				require.True(t, ok)
				require.Equal(t, errs.InvalidRequest, base)
				assert.Contains(t, errDesc(err), "transform")
				assert.Zero(t, epUc.updated)
			})
		}
	})

	t.Run("update_endpoint allows keeping existing scripts unchanged", func(t *testing.T) {
		epUc := &stubEndpointUsecase{}
		epSvc := &stubEndpointService{
			items: map[string]*endpointModel.Endpoint{
				"ep-1": {
					Id:    "ep-1",
					AppId: "app-1",
					Transform: endpointModel.Transform{
						Request:  "return { body: req.body };",
						Response: "return { body: res.body };",
					},
				},
			},
		}
		uc := &Usecase{endpointUc: epUc, endpointSvc: epSvc}
		res := &model.Result{Created: map[string]string{}}
		err := uc.applyOp(ctx, &model.Op{Op: model.OpUpdateEndpoint, Id: "ep-1"}, map[string]any{
			"active": false,
			"transform": map[string]any{
				"request":     "  return { body: req.body };\n",
				"response":    "return { body: res.body };",
				"max_workers": 8,
			},
		}, resolveApp, res, 0)
		require.NoError(t, err)
		assert.Equal(t, 1, epUc.updated)
	})
}

func TestRenderLeaf_NoTruncation(t *testing.T) {
	uc := &Usecase{}
	longValue := strings.Repeat("a", 2500) + "HIDDEN_PAYLOAD"
	got := uc.renderLeaf(context.Background(), uc.newObjectCache(), longValue, false)
	require.Equal(t, longValue, got)
}
