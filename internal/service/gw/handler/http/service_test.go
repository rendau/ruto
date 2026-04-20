package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rendau/ruto/internal/model/config"
)

func TestServiceBuild_ProxyByConfig(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Backend-Method", r.Method)
		w.Header().Set("X-Backend-Path", r.URL.Path)
		w.Header().Set("X-Backend-Query", r.URL.RawQuery)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer backend.Close()

	s := New()
	err := s.Build(&config.Root{
		PublicBaseUrl: "https://public.example",
		Apps: []*config.App{
			{
				PublicPathPrefix: "api",
				Backend: config.AppBackend{
					UrlStr: backend.URL + "/svc",
				},
				Endpoints: []*config.Endpoint{
					{
						Method: http.MethodGet,
						Path:   "users",
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "https://public.example/api/users?q=1", nil)
	rw := httptest.NewRecorder()

	s.ServeHTTP(rw, req)

	if got, want := rw.Code, http.StatusNoContent; got != want {
		t.Fatalf("unexpected response status: got %d want %d", got, want)
	}
	if got, want := rw.Header().Get("X-Backend-Method"), http.MethodGet; got != want {
		t.Fatalf("unexpected backend method: got %q want %q", got, want)
	}
	if got, want := rw.Header().Get("X-Backend-Path"), "/svc/users"; got != want {
		t.Fatalf("unexpected backend path: got %q want %q", got, want)
	}
	if got, want := rw.Header().Get("X-Backend-Query"), "q=1"; got != want {
		t.Fatalf("unexpected backend query: got %q want %q", got, want)
	}
}

func TestServiceBuild_DuplicateRoute(t *testing.T) {
	s := New()

	err := s.Build(&config.Root{
		PublicBaseUrl: "https://public.example",
		Apps: []*config.App{
			{
				PublicPathPrefix: "api",
				Backend: config.AppBackend{
					UrlStr: "http://example.local/svc",
				},
				Endpoints: []*config.Endpoint{
					{
						Method: http.MethodGet,
						Path:   "users",
					},
					{
						Method: http.MethodGet,
						Path:   "users",
					},
				},
			},
		},
	})

	if err == nil {
		t.Fatalf("Build() error = %v, want errDuplicateRoute", err)
	}
}
