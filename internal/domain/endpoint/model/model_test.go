package model

import "testing"

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

func TestEndpointNormalize_RejectWildcardInPath(t *testing.T) {
	item := &Endpoint{
		Type: TypeHTTP,
		Http: Http{
			Method: "GET",
			Path:   "doc/*",
		},
		Backend: Backend{
			CustomPath: "",
		},
	}

	err := item.Normalize()
	if err == nil {
		t.Fatalf("Normalize() expected error, got nil")
	}
	if err.Error() != "http: path: wildcard '*' is not allowed" {
		t.Fatalf("unexpected error: %v", err)
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
