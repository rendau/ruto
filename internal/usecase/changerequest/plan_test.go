package changerequest

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rendau/ruto/internal/domain/changerequest/model"
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
