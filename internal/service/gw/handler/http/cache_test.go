package http

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rendau/ruto/internal/constant"
	appModel "github.com/rendau/ruto/internal/domain/app/model"
	authModel "github.com/rendau/ruto/internal/domain/auth/model"
	endpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
	rootModel "github.com/rendau/ruto/internal/domain/root/model"
	"github.com/rendau/ruto/internal/service/gw/handler/http/proxy"
	"github.com/rendau/ruto/internal/service/gw/service/cache"
)

// newCacheTestService proxies /app/* to a backend that answers with a counter
// of the requests it got, and with the headers set by respHeader.
func newCacheTestService(t *testing.T, ep *endpointModel.Endpoint, respHeader http.Header) (*Service, *atomic.Int64) {
	t.Helper()

	var hits atomic.Int64
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := hits.Add(1)
		for k, vs := range respHeader {
			w.Header()[k] = vs
		}
		if r.URL.Query().Get("status") != "" {
			status, _ := strconv.Atoi(r.URL.Query().Get("status"))
			w.WriteHeader(status)
		}
		_, _ = w.Write([]byte("resp-" + strconv.FormatInt(n, 10)))
	}))
	t.Cleanup(backend.Close)

	ep.Id = "ep1"
	ep.Active = true
	ep.Type = endpointModel.TypeHTTP
	ep.Http = endpointModel.Http{Method: "*", Path: "*"}

	snapshot := rootModel.NewEmpty()
	snapshot.Apps = []*appModel.App{{
		Id:            "app1",
		Active:        true,
		PathPrefix:    "/app",
		Name:          "app",
		AllowWildcard: true,
		Backend:       appModel.Backend{Url: backend.URL},
		Endpoints:     []*endpointModel.Endpoint{ep},
	}}
	require.NoError(t, snapshot.Normalize())
	snapshot.InheritDown()
	snapshot.Interpolate()

	service, err := New(snapshot, proxy.NewTransport(0), cache.New(1<<20))
	require.NoError(t, err)

	return service, &hits
}

func serverCacheEndpoint() *endpointModel.Endpoint {
	return &endpointModel.Endpoint{
		Cache: endpointModel.Cache{Server: endpointModel.CacheServer{Enabled: true, Ttl: 60}},
	}
}

func TestServerCache_HitAndMiss(t *testing.T) {
	service, hits := newCacheTestService(t, serverCacheEndpoint(), nil)

	rec := performRequest(service, http.MethodGet, "/app/items?a=1&b=2", nil)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "resp-1", rec.Body.String())
	assert.Equal(t, "MISS", rec.Header().Get("X-Cache"))

	// same query in another order is the same request
	rec = performRequest(service, http.MethodGet, "/app/items?b=2&a=1", nil)
	assert.Equal(t, "resp-1", rec.Body.String())
	assert.Equal(t, "HIT", rec.Header().Get("X-Cache"))
	assert.NotEmpty(t, rec.Header().Get("Age"))

	rec = performRequest(service, http.MethodGet, "/app/items?a=2", nil)
	assert.Equal(t, "resp-2", rec.Body.String())

	rec = performRequest(service, http.MethodGet, "/app/other", nil)
	assert.Equal(t, "resp-3", rec.Body.String())

	// other methods always go to the backend
	rec = performRequest(service, http.MethodPost, "/app/items?a=1&b=2", nil)
	assert.Equal(t, "resp-4", rec.Body.String())
	assert.Empty(t, rec.Header().Get("X-Cache"))

	assert.Equal(t, int64(4), hits.Load())
}

func TestServerCache_NotCacheable(t *testing.T) {
	tests := []struct {
		name       string
		ep         func() *endpointModel.Endpoint
		respHeader http.Header
		path       string
	}{
		{name: "error status", ep: serverCacheEndpoint, path: "/app/x?status=500"},
		{name: "set-cookie", ep: serverCacheEndpoint, respHeader: http.Header{"Set-Cookie": {"s=1"}}, path: "/app/x"},
		{name: "no-store", ep: serverCacheEndpoint, respHeader: http.Header{"Cache-Control": {"no-store"}}, path: "/app/x"},
		{name: "private", ep: serverCacheEndpoint, respHeader: http.Header{"Cache-Control": {"private, max-age=10"}}, path: "/app/x"},
		{
			name: "body over the limit",
			ep: func() *endpointModel.Endpoint {
				ep := serverCacheEndpoint()
				ep.Cache.Server.MaxBodySize = 3
				return ep
			},
			path: "/app/x",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, hits := newCacheTestService(t, tt.ep(), tt.respHeader)

			first := performRequest(service, http.MethodGet, tt.path, nil)
			second := performRequest(service, http.MethodGet, tt.path, nil)

			assert.Equal(t, "resp-1", first.Body.String(), "the client gets the full response anyway")
			assert.Equal(t, "resp-2", second.Body.String())
			assert.Equal(t, int64(2), hits.Load())
		})
	}
}

func TestServerCache_KeyHeaders(t *testing.T) {
	ep := serverCacheEndpoint()
	ep.Cache.Server.VaryHeaders = []string{"accept-language"}
	ep.Auth = authModel.Auth{
		Enabled: true,
		Mode:    constant.AuthModeReplace,
		Methods: []*authModel.AuthMethod{{
			APIKey: &authModel.AuthMethodAPIKey{
				Keys: []authModel.AuthMethodAPIKeyItem{{Name: "a", Key: "key-a"}, {Name: "b", Key: "key-b"}},
			},
		}},
	}
	service, hits := newCacheTestService(t, ep, nil)

	userA := http.Header{"X-Api-Key": {"key-a"}}
	userB := http.Header{"X-Api-Key": {"key-b"}}

	assert.Equal(t, "resp-1", performRequest(service, http.MethodGet, "/app/me", userA).Body.String())
	assert.Equal(t, "resp-1", performRequest(service, http.MethodGet, "/app/me", userA).Body.String())
	assert.Equal(t, "resp-2", performRequest(service, http.MethodGet, "/app/me", userB).Body.String(), "a user never gets another user's response")

	userAKz := http.Header{"X-Api-Key": {"key-a"}, "Accept-Language": {"kk"}}
	assert.Equal(t, "resp-3", performRequest(service, http.MethodGet, "/app/me", userAKz).Body.String())

	// auth still runs on a cached path
	assert.Equal(t, http.StatusUnauthorized, performRequest(service, http.MethodGet, "/app/me", nil).Code)

	assert.Equal(t, int64(3), hits.Load())
}

func TestServerCache_Shared(t *testing.T) {
	ep := serverCacheEndpoint()
	ep.Cache.Server.Shared = true
	ep.Auth = authModel.Auth{
		Enabled: true,
		Mode:    constant.AuthModeReplace,
		Methods: []*authModel.AuthMethod{{
			APIKey: &authModel.AuthMethodAPIKey{
				Keys: []authModel.AuthMethodAPIKeyItem{{Name: "a", Key: "key-a"}, {Name: "b", Key: "key-b"}},
			},
		}},
	}
	service, hits := newCacheTestService(t, ep, nil)

	assert.Equal(t, "resp-1", performRequest(service, http.MethodGet, "/app/list", http.Header{"X-Api-Key": {"key-a"}}).Body.String())
	rec := performRequest(service, http.MethodGet, "/app/list", http.Header{"X-Api-Key": {"key-b"}})
	assert.Equal(t, "resp-1", rec.Body.String(), "a shared cache answers every caller")
	assert.Equal(t, "HIT", rec.Header().Get("X-Cache"))

	// auth still runs before the shared cache
	assert.Equal(t, http.StatusUnauthorized, performRequest(service, http.MethodGet, "/app/list", nil).Code)
	assert.Equal(t, http.StatusUnauthorized, performRequest(service, http.MethodGet, "/app/list", http.Header{"X-Api-Key": {"wrong"}}).Code)

	assert.Equal(t, int64(1), hits.Load())
}

func TestBrowserCache(t *testing.T) {
	tests := []struct {
		name       string
		browser    endpointModel.CacheBrowser
		respHeader http.Header
		path       string
		method     string
		want       string
	}{
		{
			name:    "private by default",
			browser: endpointModel.CacheBrowser{Enabled: true, MaxAge: 300},
			path:    "/app/x", method: http.MethodGet,
			want: "private, max-age=300",
		},
		{
			name:    "public",
			browser: endpointModel.CacheBrowser{Enabled: true, Scope: constant.CacheScopePublic, MaxAge: 60},
			path:    "/app/x", method: http.MethodHead,
			want: "public, max-age=60",
		},
		{
			name:    "no-store",
			browser: endpointModel.CacheBrowser{Enabled: true, Scope: constant.CacheScopeNoStore, MaxAge: 60},
			path:    "/app/x", method: http.MethodGet,
			want: "no-store",
		},
		{
			name:       "backend value kept",
			browser:    endpointModel.CacheBrowser{Enabled: true, MaxAge: 300},
			respHeader: http.Header{"Cache-Control": {"no-cache"}},
			path:       "/app/x", method: http.MethodGet,
			want: "no-cache",
		},
		{
			name:       "backend value overridden",
			browser:    endpointModel.CacheBrowser{Enabled: true, MaxAge: 300, Override: true},
			respHeader: http.Header{"Cache-Control": {"no-cache"}},
			path:       "/app/x", method: http.MethodGet,
			want: "private, max-age=300",
		},
		{
			name:    "error response untouched",
			browser: endpointModel.CacheBrowser{Enabled: true, MaxAge: 300},
			path:    "/app/x?status=404", method: http.MethodGet,
			want: "",
		},
		{
			name:    "post untouched",
			browser: endpointModel.CacheBrowser{Enabled: true, MaxAge: 300},
			path:    "/app/x", method: http.MethodPost,
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, _ := newCacheTestService(t, &endpointModel.Endpoint{Cache: endpointModel.Cache{Browser: tt.browser}}, tt.respHeader)

			rec := performRequest(service, tt.method, tt.path, nil)

			assert.Equal(t, tt.want, rec.Header().Get("Cache-Control"))
		})
	}
}

func TestBrowserCache_OnServerCacheHit(t *testing.T) {
	ep := serverCacheEndpoint()
	ep.Cache.Browser = endpointModel.CacheBrowser{Enabled: true, MaxAge: 30}
	service, _ := newCacheTestService(t, ep, nil)

	performRequest(service, http.MethodGet, "/app/x", nil)
	rec := performRequest(service, http.MethodGet, "/app/x", nil)

	assert.Equal(t, "HIT", rec.Header().Get("X-Cache"))
	assert.Equal(t, "private, max-age=30", rec.Header().Get("Cache-Control"))
}
