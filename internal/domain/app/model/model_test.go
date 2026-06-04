package model

import (
	"testing"
)

func TestAppNormalize_RejectWildcardInPathPrefix(t *testing.T) {
	item := &App{
		PathPrefix: "api/*",
		Backend: Backend{
			Url: "http://example.local/svc",
		},
	}

	err := item.Normalize()
	if err == nil {
		t.Fatalf("Normalize() expected error, got nil")
	}
	if err.Error() != "path_prefix: invalid format" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAppNormalize_RejectPathParamsInPathPrefix(t *testing.T) {
	item := &App{
		PathPrefix: "api/{id}",
		Backend: Backend{
			Url: "http://example.local/svc",
		},
	}

	err := item.Normalize()
	if err == nil {
		t.Fatalf("Normalize() expected error, got nil")
	}
	if err.Error() != "path_prefix: invalid format" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAppNormalize_RejectRegexParamsInPathPrefix(t *testing.T) {
	item := &App{
		PathPrefix: "api/{id:[0-9]+}",
		Backend: Backend{
			Url: "http://example.local/svc",
		},
	}

	err := item.Normalize()
	if err == nil {
		t.Fatalf("Normalize() expected error, got nil")
	}
	if err.Error() != "path_prefix: invalid format" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAppNormalize_AllowSimplePathPrefix(t *testing.T) {
	item := &App{
		PathPrefix: "api/v1_public-1",
		Backend: Backend{
			Url: "http://example.local/svc",
		},
	}

	err := item.Normalize()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if item.PathPrefix != "/api/v1_public-1" {
		t.Fatalf("unexpected normalized path_prefix: %q", item.PathPrefix)
	}
}

func TestAppNormalize_RejectInvalidSwaggerURLScheme(t *testing.T) {
	item := &App{
		PathPrefix: "api",
		Backend: Backend{
			Url:        "http://example.local/svc",
			SwaggerUrl: "ftp://example.local/swagger.json",
		},
	}

	err := item.Normalize()
	if err == nil {
		t.Fatalf("Normalize() expected error, got nil")
	}
	if err.Error() != "backend: swagger_url: scheme must be http or https" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAppNormalize_AllowValidSwaggerURL(t *testing.T) {
	item := &App{
		PathPrefix: "api",
		Backend: Backend{
			Url:        "http://example.local/svc",
			SwaggerUrl: "https://example.local/openapi.json",
		},
	}

	err := item.Normalize()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if item.Backend.SwaggerUrl != "https://example.local/openapi.json" {
		t.Fatalf("unexpected normalized swagger_url: %q", item.Backend.SwaggerUrl)
	}
}

func TestAppNormalize_BackendRequestParams(t *testing.T) {
	item := &App{
		PathPrefix: "api",
		Backend: Backend{
			Url: "http://example.local/svc",
			Headers: map[string]string{
				" X-App-Token ": " secret ",
				" ":             "ignored",
			},
			QueryParams: map[string]string{
				" tenant ": " acme ",
			},
		},
	}

	err := item.Normalize()
	if err == nil {
		t.Fatalf("Normalize() expected error, got nil")
	}
	if err.Error() != "backend: headers: empty key after normalization" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAppNormalize_BackendRequestParams_Valid(t *testing.T) {
	item := &App{
		PathPrefix: "api",
		Backend: Backend{
			Url: "http://example.local/svc",
			Headers: map[string]string{
				" X-App-Token ": " secret ",
			},
			QueryParams: map[string]string{
				" tenant ": " acme ",
			},
		},
	}

	err := item.Normalize()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if item.Backend.Headers["X-App-Token"] != "secret" {
		t.Fatalf("header normalize failed: %#v", item.Backend.Headers)
	}
	if item.Backend.QueryParams["tenant"] != "acme" {
		t.Fatalf("query param normalize failed: %#v", item.Backend.QueryParams)
	}
}
