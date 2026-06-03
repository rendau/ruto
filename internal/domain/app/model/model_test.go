package model

import (
	"testing"

	endpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
	variableModel "github.com/rendau/ruto/internal/domain/variable/model"
)

func TestAppNormalize_RejectWildcardInPathPrefix(t *testing.T) {
	item := &App{
		PathPrefix: "api/*",
		Backend: AppBackend{
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
		Backend: AppBackend{
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
		Backend: AppBackend{
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
		Backend: AppBackend{
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
		Backend: AppBackend{
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
		Backend: AppBackend{
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
		Backend: AppBackend{
			Url: "http://example.local/svc",
			Headers: map[string]string{
				" X-App-Token ": " secret ",
				" ":             "ignored",
			},
			QueryParams: map[string]string{
				" tenant ": " acme ",
				"":         "ignored",
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
	if _, ok := item.Backend.Headers[""]; ok {
		t.Fatalf("empty header key was not removed: %#v", item.Backend.Headers)
	}
	if item.Backend.QueryParams["tenant"] != "acme" {
		t.Fatalf("query param normalize failed: %#v", item.Backend.QueryParams)
	}
}

func TestAppBackendRequestParamsWithVariables(t *testing.T) {
	item := &App{
		Backend: AppBackend{
			Headers: map[string]string{
				"X-App-Token": "{{token}}",
			},
			QueryParams: map[string]string{
				"{{tenant_param}}": "{{tenant}}",
			},
		},
	}
	endpoint := &endpointModel.Endpoint{
		Backend: endpointModel.Backend{
			Headers: map[string]string{
				"X-Endpoint-Token": "{{composed}}",
			},
		},
	}

	params, err := item.BackendRequestParamsWithVariables(endpoint, []variableModel.Variable{
		{Key: "token", Value: "endpoint-token"},
		{Key: "tenant", Value: "acme"},
		{Key: "tenant_param", Value: "tenant"},
		{Key: "composed", Value: "{{token}}:{{tenant}}"},
	})
	if err != nil {
		t.Fatalf("BackendRequestParamsWithVariables() unexpected error: %v", err)
	}
	if params.Headers["X-App-Token"] != "endpoint-token" {
		t.Fatalf("app header interpolation failed: %#v", params.Headers)
	}
	if params.Headers["X-Endpoint-Token"] != "endpoint-token:acme" {
		t.Fatalf("endpoint header interpolation failed: %#v", params.Headers)
	}
	if params.QueryParams["tenant"] != "acme" {
		t.Fatalf("query interpolation failed: %#v", params.QueryParams)
	}
}

func TestAppGrpcAddress_ParsesBackendURLWhenLoadedFromStorage(t *testing.T) {
	item := &App{
		Backend: AppBackend{
			Url:      "http://zeon-lb-tcp",
			GrpcPort: 9200,
		},
	}

	if item.Backend.ParsedUrl != nil {
		t.Fatalf("test setup expected ParsedUrl to be nil")
	}
	if got := item.GrpcAddress(); got != "zeon-lb-tcp:9200" {
		t.Fatalf("unexpected grpc address: %q", got)
	}
}
