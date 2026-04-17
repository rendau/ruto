package http

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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
		Apps: []config.App{
			{
				Id:               "users-app",
				PublicPathPrefix: "api",
				Backend: config.AppBackend{
					Host: backend.URL,
					Path: "svc",
				},
				Endpoints: []config.Endpoint{
					{
						Id:     "users-list",
						Method: http.MethodGet,
						Path:   "users",
						Backend: config.EndpointBackend{
							Path: "users/list",
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/users?q=1", nil)
	rw := httptest.NewRecorder()

	s.ServeHTTP(rw, req)

	if got, want := rw.Code, http.StatusNoContent; got != want {
		t.Fatalf("unexpected response status: got %d want %d", got, want)
	}
	if got, want := rw.Header().Get("X-Backend-Method"), http.MethodGet; got != want {
		t.Fatalf("unexpected backend method: got %q want %q", got, want)
	}
	if got, want := rw.Header().Get("X-Backend-Path"), "/svc/users/list"; got != want {
		t.Fatalf("unexpected backend path: got %q want %q", got, want)
	}
	if got, want := rw.Header().Get("X-Backend-Query"), "q=1"; got != want {
		t.Fatalf("unexpected backend query: got %q want %q", got, want)
	}
}

func TestServiceBuild_IPValidationMiddleware(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer backend.Close()

	s := New()
	err := s.Build(&config.Root{
		PublicBaseUrl: "https://public.example",
		Apps: []config.App{
			{
				Id:               "ip-app",
				PublicPathPrefix: "api",
				Backend: config.AppBackend{
					Host: backend.URL,
					Path: "svc",
				},
				Endpoints: []config.Endpoint{
					{
						Id:     "ip-protected",
						Method: http.MethodGet,
						Path:   "private",
						Backend: config.EndpointBackend{
							Path: "private",
						},
						IpValidation: config.EndpointIpValidation{
							AllowedIps: []string{"127.0.0.1"},
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	t.Run("forbidden for unknown ip", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/private", nil)
		req.RemoteAddr = "10.1.1.1:1234"
		rw := httptest.NewRecorder()

		s.ServeHTTP(rw, req)

		if got, want := rw.Code, http.StatusForbidden; got != want {
			t.Fatalf("unexpected response status: got %d want %d", got, want)
		}
	})

	t.Run("allowed for whitelisted ip", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/private", nil)
		req.RemoteAddr = "127.0.0.1:2222"
		rw := httptest.NewRecorder()

		s.ServeHTTP(rw, req)

		if got, want := rw.Code, http.StatusNoContent; got != want {
			t.Fatalf("unexpected response status: got %d want %d", got, want)
		}
	})
}

func TestServiceBuild_CorsPreflight(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer backend.Close()

	s := New()
	err := s.Build(&config.Root{
		PublicBaseUrl: "https://public.example",
		Cors: config.RootCors{
			Enabled:          true,
			AllowCredentials: true,
			MaxAge:           "3600",
			AllowOrigins:     []string{"https://app.example"},
			AllowMethods:     []string{"GET", "POST"},
			AllowHeaders:     []string{"Authorization", "Content-Type"},
		},
		Apps: []config.App{
			{
				Id:               "cors-app",
				PublicPathPrefix: "api",
				Backend: config.AppBackend{
					Host: backend.URL,
					Path: "svc",
				},
				Endpoints: []config.Endpoint{
					{
						Id:     "cors-endpoint",
						Method: http.MethodGet,
						Path:   "users",
						Backend: config.EndpointBackend{
							Path: "users",
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodOptions, "/api/users", nil)
	req.Header.Set("Origin", "https://app.example")
	req.Header.Set("Access-Control-Request-Method", "GET")
	req.Header.Set("Access-Control-Request-Headers", "Authorization")
	rw := httptest.NewRecorder()

	s.ServeHTTP(rw, req)

	if got, want := rw.Code, http.StatusNoContent; got != want {
		t.Fatalf("unexpected response status: got %d want %d", got, want)
	}
	if got, want := rw.Header().Get("Access-Control-Allow-Origin"), "https://app.example"; got != want {
		t.Fatalf("unexpected allow origin: got %q want %q", got, want)
	}
	if got := rw.Header().Get("Access-Control-Allow-Methods"); !strings.Contains(got, "GET") {
		t.Fatalf("unexpected allow methods: %q", got)
	}
	if got, want := rw.Header().Get("Access-Control-Max-Age"), "3600"; got != want {
		t.Fatalf("unexpected max age: got %q want %q", got, want)
	}
}

func TestServiceBuild_DuplicateRoute(t *testing.T) {
	s := New()
	err := s.Build(&config.Root{
		PublicBaseUrl: "https://public.example",
		Apps: []config.App{
			{
				Id:               "app-1",
				PublicPathPrefix: "api",
				Backend: config.AppBackend{
					Host: "http://example.local",
					Path: "svc",
				},
				Endpoints: []config.Endpoint{
					{
						Id:     "ep-1",
						Method: http.MethodGet,
						Path:   "users",
						Backend: config.EndpointBackend{
							Path: "users/one",
						},
					},
				},
			},
			{
				Id:               "app-2",
				PublicPathPrefix: "api",
				Backend: config.AppBackend{
					Host: "http://example.local",
					Path: "svc",
				},
				Endpoints: []config.Endpoint{
					{
						Id:     "ep-2",
						Method: http.MethodGet,
						Path:   "users",
						Backend: config.EndpointBackend{
							Path: "users/two",
						},
					},
				},
			},
		},
	})
	if err == nil {
		t.Fatal("expected duplicate route error")
	}
	if !strings.Contains(err.Error(), "duplicate route") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestServiceBuild_TimeoutMiddleware(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(30 * time.Millisecond)
		_, _ = io.WriteString(w, "ok")
	}))
	defer backend.Close()

	s := New()
	err := s.Build(&config.Root{
		PublicBaseUrl: "https://public.example",
		Timeout:       config.RootTimeout{Global: 5 * time.Millisecond},
		Apps: []config.App{
			{
				Id:               "timeout-app",
				PublicPathPrefix: "api",
				Backend: config.AppBackend{
					Host: backend.URL,
					Path: "svc",
				},
				Endpoints: []config.Endpoint{
					{
						Id:     "timeout-endpoint",
						Method: http.MethodGet,
						Path:   "users",
						Backend: config.EndpointBackend{
							Path: "users",
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	rw := httptest.NewRecorder()

	s.ServeHTTP(rw, req)

	if got, want := rw.Code, http.StatusServiceUnavailable; got != want {
		t.Fatalf("unexpected response status: got %d want %d", got, want)
	}
}
