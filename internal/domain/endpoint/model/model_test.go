package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEndpointNormalize_AllowEmptyPath(t *testing.T) {
	item := &Endpoint{
		Type: TypeHTTP,
		Http: Http{
			Method: "get",
			Path:   "",
		},
		Backend: Backend{
			CustomPath: "",
		},
	}

	if err := item.Normalize(); err != nil {
		t.Fatalf("Normalize() unexpected error: %v", err)
	}
	if item.Http.Method != "GET" {
		t.Fatalf("method normalize failed, got %q", item.Http.Method)
	}
	if item.Http.Path != "" {
		t.Fatalf("path normalize failed, expected empty, got %q", item.Http.Path)
	}
}

func TestEndpointNormalize_SlashPathToEmpty(t *testing.T) {
	item := &Endpoint{
		Type: TypeHTTP,
		Http: Http{
			Method: "POST",
			Path:   "/",
		},
		Backend: Backend{
			CustomPath: "",
		},
	}

	if err := item.Normalize(); err != nil {
		t.Fatalf("Normalize() unexpected error: %v", err)
	}
	if item.Http.Path != "" {
		t.Fatalf("path normalize failed, expected empty, got %q", item.Http.Path)
	}
}

func TestEndpointNormalize_WildcardPath(t *testing.T) {
	tests := []struct {
		path       string
		customPath string
		wantErr    string
		wantHas    bool
	}{
		{path: "doc/*", wantHas: true},
		{path: "/doc/*/", wantHas: true},
		{path: "*", wantHas: true},
		{path: "doc/{id}/*", wantHas: true},
		{path: "doc"},
		{path: "doc/*/edit", wantErr: "http: path: wildcard '*' is allowed only as the last segment"},
		{path: "doc/a*", wantErr: "http: path: wildcard '*' is allowed only as the last segment"},
		{path: "doc/**", wantErr: "http: path: wildcard '*' is allowed only as the last segment"},
		{path: "*/*", wantErr: "http: path: wildcard '*' is allowed only as the last segment"},
		{path: "doc/{id:[0-9]*}", wantErr: "http: path: wildcard '*' is allowed only as the last segment"},
		{path: "doc/*", customPath: "internal/doc", wantErr: "backend: custom_path: not allowed with wildcard path"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			item := &Endpoint{
				Type:    TypeHTTP,
				Http:    Http{Method: "GET", Path: tt.path},
				Backend: Backend{CustomPath: tt.customPath},
			}

			err := item.Normalize()
			if tt.wantErr != "" {
				require.EqualError(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.wantHas, item.Http.HasWildcard())
		})
	}
}

func TestEndpointNormalize_BackendRequestParams(t *testing.T) {
	item := &Endpoint{
		Type: TypeHTTP,
		Http: Http{
			Method: "GET",
			Path:   "doc",
		},
		Backend: Backend{
			Headers: map[string]string{
				" X-Endpoint-Token ": " secret ",
			},
			QueryParams: map[string]string{
				" mode ": " full ",
			},
		},
	}

	if err := item.Normalize(); err != nil {
		t.Fatalf("Normalize() unexpected error: %v", err)
	}
	if item.Backend.Headers["X-Endpoint-Token"] != "secret" {
		t.Fatalf("header normalize failed: %#v", item.Backend.Headers)
	}
	if item.Backend.QueryParams["mode"] != "full" {
		t.Fatalf("query param normalize failed: %#v", item.Backend.QueryParams)
	}
}

func TestEndpointNormalize_TrackPathsOnlyForWildcard(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{path: "docs/*", want: true},
		{path: "*", want: true},
		{path: "docs/{id}"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			item := &Endpoint{
				Type:       TypeHTTP,
				TrackPaths: true,
				Http:       Http{Method: "GET", Path: tt.path},
			}
			require.NoError(t, item.Normalize())
			require.Equal(t, tt.want, item.TrackPaths)
		})
	}

	item := &Endpoint{
		Type:       TypeGRPC,
		TrackPaths: true,
		Grpc:       Grpc{Service: "svc.Users", Method: "Get", Path: "/svc.Users/Get"},
	}
	require.NoError(t, item.Normalize())
	require.False(t, item.TrackPaths)
}

func TestEndpointNormalize_Cache(t *testing.T) {
	item := &Endpoint{
		Type: TypeHTTP,
		Http: Http{Method: "GET", Path: "items"},
		Cache: Cache{
			Server:  CacheServer{Enabled: true, Ttl: 60, VaryHeaders: []string{" accept-language", "", "Accept-Language", "x-tenant"}},
			Browser: CacheBrowser{Enabled: true, MaxAge: 30},
		},
	}
	require.NoError(t, item.Normalize())
	require.Equal(t, []string{"Accept-Language", "X-Tenant"}, item.Cache.Server.VaryHeaders)
	require.Equal(t, "private, max-age=30", item.Cache.Browser.HeaderValue())

	item.Cache.Server.Ttl = 0
	require.ErrorContains(t, item.Normalize(), "cache: server: ttl")

	item.Cache.Server.Ttl = 60
	item.Cache.Browser.Scope = "everyone"
	require.ErrorContains(t, item.Normalize(), "cache: browser: scope")

	grpc := &Endpoint{
		Type:  TypeGRPC,
		Grpc:  Grpc{Service: "pkg.Svc", Method: "Get", Path: "/pkg.Svc/Get"},
		Cache: Cache{Server: CacheServer{Enabled: true}},
	}
	require.NoError(t, grpc.Normalize())
	require.False(t, grpc.Cache.Server.Enabled, "cache is http only")
}
