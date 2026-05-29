package swagger

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestService_LoadEndpoints(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"paths": {
				"/users": {
					"get": {},
					"post": {},
					"parameters": []
				},
				"/status/": {
					"get": {}
				},
				"/trace": {
					"trace": {}
				},
				"/ws": {
					"subscribe": {}
				}
			}
		}`))
	}))
	defer server.Close()

	svc := New(2 * time.Second)
	result, err := svc.LoadEndpoints(context.Background(), server.URL)
	require.NoError(t, err)
	require.Equal(t, []Endpoint{
		{Method: "GET", Path: "/status"},
		{Method: "TRACE", Path: "/trace"},
		{Method: "GET", Path: "/users"},
		{Method: "POST", Path: "/users"},
	}, result)
}

func TestService_LoadEndpoints_YAML(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/yaml")
		_, _ = w.Write([]byte(`
paths:
  /users:
    get: {}
    post: {}
    parameters: []
  /status/:
    get: {}
  /trace:
    trace: {}
  /ws:
    subscribe: {}
`))
	}))
	defer server.Close()

	svc := New(2 * time.Second)
	result, err := svc.LoadEndpoints(context.Background(), server.URL)
	require.NoError(t, err)
	require.Equal(t, []Endpoint{
		{Method: "GET", Path: "/status"},
		{Method: "TRACE", Path: "/trace"},
		{Method: "GET", Path: "/users"},
		{Method: "POST", Path: "/users"},
	}, result)
}
