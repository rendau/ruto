package transform

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func mustNew(t *testing.T, script string) *Transformer {
	t.Helper()
	tr, err := New(script, 4)
	require.NoError(t, err)
	return tr
}

func transform(t *testing.T, tr *Transformer, in *Request) (*Result, error) {
	t.Helper()
	return tr.Transform(context.Background(), in)
}

func TestPassthroughWhenFieldsOmitted(t *testing.T) {
	tr := mustNew(t, `return { headers: { ...req.headers, "X-Tenant": req.vars["tenant"] } };`)

	res, err := transform(t, tr, &Request{
		Method:  "POST",
		Path:    "/orders",
		Headers: map[string][]string{"Content-Type": {"application/json"}},
		Params:  map[string][]string{"q": {"1"}},
		Body:    []byte(`{"a":1}`),
		Vars:    map[string]string{"tenant": "acme"},
	})
	require.NoError(t, err)

	require.Nil(t, res.Method, "method omitted -> nil")
	require.Nil(t, res.Path, "path omitted -> nil")
	require.Nil(t, res.Params, "params omitted -> nil")
	require.False(t, res.BodySet, "body omitted -> not set")

	require.Equal(t, []string{"application/json"}, res.Headers["Content-Type"], "spread keeps existing array")
	require.Equal(t, []string{"acme"}, res.Headers["X-Tenant"], "scalar coerced to single-value list")
}

func TestMultiValueHeadersPreserved(t *testing.T) {
	tr := mustNew(t, `
		var h = { ...req.headers };
		h["X-Multi"] = ["a", "b"];
		return { headers: h };
	`)

	res, err := transform(t, tr, &Request{
		Headers: map[string][]string{"Accept": {"text/html", "application/json"}},
	})
	require.NoError(t, err)
	require.Equal(t, []string{"text/html", "application/json"}, res.Headers["Accept"])
	require.Equal(t, []string{"a", "b"}, res.Headers["X-Multi"])
}

func TestBodyObjectIsStringified(t *testing.T) {
	tr := mustNew(t, `
		var b = req.body;
		b.source = "gateway";
		return { body: b };
	`)

	res, err := transform(t, tr, &Request{Body: []byte(`{"order_id":42}`)})
	require.NoError(t, err)
	require.True(t, res.BodySet)
	require.JSONEq(t, `{"order_id":42,"source":"gateway"}`, string(res.Body))
}

func TestBodyStringIsRaw(t *testing.T) {
	tr := mustNew(t, `return { body: req.raw_body };`)

	res, err := transform(t, tr, &Request{Body: []byte(`<xml/>`)})
	require.NoError(t, err)
	require.True(t, res.BodySet)
	require.Equal(t, "<xml/>", string(res.Body))
}

func TestEmptyArrayStaysArray(t *testing.T) {
	tr := mustNew(t, `return { body: { items: [] } };`)

	res, err := transform(t, tr, &Request{})
	require.NoError(t, err)
	require.JSONEq(t, `{"items":[]}`, string(res.Body))
}

func TestMethodAndPathOverride(t *testing.T) {
	tr := mustNew(t, `return { method: "PUT", path: "v2/handle" };`)

	res, err := transform(t, tr, &Request{Method: "POST", Path: "/orders"})
	require.NoError(t, err)
	require.NotNil(t, res.Method)
	require.Equal(t, "PUT", *res.Method)
	require.NotNil(t, res.Path)
	require.Equal(t, "v2/handle", *res.Path)
}

func TestInvalidReturnIsError(t *testing.T) {
	tr := mustNew(t, `return 42;`)

	_, err := transform(t, tr, &Request{})
	require.Error(t, err, "non-object return must fail, not reach backend")
}

func TestThrowIsError(t *testing.T) {
	tr := mustNew(t, `throw new Error("boom");`)

	_, err := transform(t, tr, &Request{})
	require.Error(t, err)
}

func TestRuntimeRecoversAfterError(t *testing.T) {
	tr := mustNew(t, `
		if (req.method === "BAD") throw new Error("boom");
		return { method: "OK" };
	`)

	_, err := transform(t, tr, &Request{Method: "BAD"})
	require.Error(t, err)

	// A dropped runtime must not poison the pool.
	res, err := transform(t, tr, &Request{Method: "GOOD"})
	require.NoError(t, err)
	require.Equal(t, "OK", *res.Method)
}

func TestNonJSONBodyParsesToUndefined(t *testing.T) {
	tr := mustNew(t, `return { body: req.body === undefined ? "no-json" : "json" };`)

	res, err := transform(t, tr, &Request{Body: []byte("not json")})
	require.NoError(t, err)
	require.Equal(t, "no-json", string(res.Body))
}

// TestBoundedPoolConcurrency drives many concurrent calls through a 2-worker
// pool: runtimes are reused across goroutines, so correctness here means the
// pool serializes access safely (run with -race).
func TestBoundedPoolConcurrency(t *testing.T) {
	tr, err := New(`return { headers: { "X-Echo": req.vars["id"] } };`, 2)
	require.NoError(t, err)

	const n = 200
	var wg sync.WaitGroup
	errs := make([]error, n)
	got := make([]string, n)

	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			id := string(rune('A' + i%26))
			res, err := tr.Transform(context.Background(), &Request{Vars: map[string]string{"id": id}})
			if err != nil {
				errs[i] = err
				return
			}
			got[i] = res.Headers["X-Echo"][0]
		}()
	}
	wg.Wait()

	for i := range n {
		require.NoError(t, errs[i])
		require.Equal(t, string(rune('A'+i%26)), got[i], "no cross-talk between pooled runtimes")
	}
}

func TestContextCanceledWhenPoolSaturated(t *testing.T) {
	tr, err := New(`return {};`, 1)
	require.NoError(t, err)

	// Occupy the only worker slot so acquisition cannot succeed.
	<-tr.tokens

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = tr.Transform(ctx, &Request{})
	require.ErrorIs(t, err, context.Canceled, "saturated pool must yield to context, not block forever")
}
