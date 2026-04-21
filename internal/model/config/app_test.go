package config

import (
	"errors"
	"testing"
)

func TestAppNormalize(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := App{
			Id:               "  app-id  ",
			PublicPathPrefix: " /public ",
			Backend: AppBackend{
				UrlStr: " https://example.com/base ",
			},
			Endpoints: []*Endpoint{
				{
					Method: " get ",
					Path:   " /v1/ping ",
					Backend: EndpointBackend{
						CustomPath: " /ping ",
					},
				},
			},
		}

		err := m.Normalize()
		if err != nil {
			t.Fatalf("Normalize() error = %v", err)
		}

		if m.Id != "app-id" {
			t.Fatalf("Id = %q, want %q", m.Id, "app-id")
		}
		if m.PublicPathPrefix != "/public" {
			t.Fatalf("PublicPathPrefix = %q, want %q", m.PublicPathPrefix, "public")
		}
		if m.Backend.Url == nil || m.Backend.Url.String() != "https://example.com/base" {
			t.Fatalf("Backend.Url = %v, want %q", m.Backend.Url, "https://example.com/base")
		}
		if m.Endpoints[0].Method != "GET" {
			t.Fatalf("Endpoints[0].Method = %q, want %q", m.Endpoints[0].Method, "GET")
		}
	})

	t.Run("error empty public path prefix", func(t *testing.T) {
		m := App{
			PublicPathPrefix: " / ",
			Backend:          AppBackend{UrlStr: "https://example.com"},
		}
		err := m.Normalize()
		if !errors.Is(err, errEmptyValue) {
			t.Fatalf("Normalize() error = %v, want errEmptyValue", err)
		}
	})

	t.Run("error endpoint normalize", func(t *testing.T) {
		m := App{
			PublicPathPrefix: "public",
			Backend:          AppBackend{UrlStr: "https://example.com"},
			Endpoints: []*Endpoint{
				{
					Method: " ",
					Path:   "/ok",
					Backend: EndpointBackend{
						CustomPath: "/ok",
					},
				},
			},
		}
		err := m.Normalize()
		if !errors.Is(err, errEmptyValue) {
			t.Fatalf("Normalize() error = %v, want errEmptyValue", err)
		}
	})
}

func TestAppBackendNormalize(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := AppBackend{UrlStr: " https://example.com/api "}
		err := m.Normalize()
		if err != nil {
			t.Fatalf("Normalize() error = %v", err)
		}
		if m.UrlStr != "https://example.com/api" {
			t.Fatalf("UrlStr = %q, want %q", m.UrlStr, "https://example.com/api")
		}
		if m.Url == nil || m.Url.String() != "https://example.com/api" {
			t.Fatalf("Url = %v, want %q", m.Url, "https://example.com/api")
		}
	})

	t.Run("error invalid url", func(t *testing.T) {
		m := AppBackend{UrlStr: "://bad"}
		err := m.Normalize()
		if err == nil {
			t.Fatal("Normalize() error = nil, want non-nil")
		}
	})

	t.Run("error empty url", func(t *testing.T) {
		m := AppBackend{UrlStr: "  "}
		err := m.Normalize()
		if !errors.Is(err, errEmptyValue) {
			t.Fatalf("Normalize() error = %v, want errEmptyValue", err)
		}
	})
}
