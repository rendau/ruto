package transform

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func mustNewResp(t *testing.T, script string) *ResponseTransformer {
	t.Helper()
	tr, err := NewResponse(script, 4)
	require.NoError(t, err)
	return tr
}

func transformResp(t *testing.T, tr *ResponseTransformer, in *Response) (*ResponseResult, error) {
	t.Helper()
	return tr.Transform(context.Background(), in)
}

func TestRespPassthroughWhenOmitted(t *testing.T) {
	tr := mustNewResp(t, `return { headers: { ...res.headers, "X-Gw": "1" } };`)

	out, err := transformResp(t, tr, &Response{
		Status:  201,
		Headers: map[string][]string{"Content-Type": {"application/json"}},
		Body:    []byte(`{"id":1}`),
	})
	require.NoError(t, err)

	require.Nil(t, out.Status, "status omitted -> nil")
	require.False(t, out.BodySet, "body omitted -> not set")
	require.Equal(t, []string{"application/json"}, out.Headers["Content-Type"])
	require.Equal(t, []string{"1"}, out.Headers["X-Gw"])
}

func TestRespStatusOverride(t *testing.T) {
	tr := mustNewResp(t, `return { status: res.status === 404 ? 200 : res.status };`)

	out, err := transformResp(t, tr, &Response{Status: 404})
	require.NoError(t, err)
	require.NotNil(t, out.Status)
	require.Equal(t, 200, *out.Status)
}

func TestRespBodyUnwrap(t *testing.T) {
	// Unwrap a backend envelope { data: ... } into just its data.
	tr := mustNewResp(t, `return { body: res.body.data };`)

	out, err := transformResp(t, tr, &Response{Body: []byte(`{"data":{"id":7},"meta":1}`)})
	require.NoError(t, err)
	require.True(t, out.BodySet)
	require.JSONEq(t, `{"id":7}`, string(out.Body))
}

func TestRespBodyStringIsRaw(t *testing.T) {
	tr := mustNewResp(t, `return { body: res.raw_body.toUpperCase() };`)

	out, err := transformResp(t, tr, &Response{Body: []byte(`ok`)})
	require.NoError(t, err)
	require.Equal(t, "OK", string(out.Body))
}

func TestRespNonNumericStatusIsError(t *testing.T) {
	tr := mustNewResp(t, `return { status: "abc" };`)

	_, err := transformResp(t, tr, &Response{Status: 200})
	require.Error(t, err, "non-numeric status must fail, not reach client")
}

func TestRespThrowIsError(t *testing.T) {
	tr := mustNewResp(t, `throw new Error("boom");`)

	_, err := transformResp(t, tr, &Response{Status: 200})
	require.Error(t, err)
}
