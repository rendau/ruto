package model

import "testing"

func TestEndpointNormalize_AllowEmptyPath(t *testing.T) {
	item := &Endpoint{
		Method: "get",
		Path:   "",
		Backend: Backend{
			CustomPath: "",
		},
	}

	if err := item.Normalize(); err != nil {
		t.Fatalf("Normalize() unexpected error: %v", err)
	}
	if item.Method != "GET" {
		t.Fatalf("method normalize failed, got %q", item.Method)
	}
	if item.Path != "" {
		t.Fatalf("path normalize failed, expected empty, got %q", item.Path)
	}
}

func TestEndpointNormalize_SlashPathToEmpty(t *testing.T) {
	item := &Endpoint{
		Method: "POST",
		Path:   "/",
		Backend: Backend{
			CustomPath: "",
		},
	}

	if err := item.Normalize(); err != nil {
		t.Fatalf("Normalize() unexpected error: %v", err)
	}
	if item.Path != "" {
		t.Fatalf("path normalize failed, expected empty, got %q", item.Path)
	}
}

func TestEndpointNormalize_RejectWildcardInPath(t *testing.T) {
	item := &Endpoint{
		Method: "GET",
		Path:   "doc/*",
		Backend: Backend{
			CustomPath: "",
		},
	}

	err := item.Normalize()
	if err == nil {
		t.Fatalf("Normalize() expected error, got nil")
	}
	if err.Error() != "path: wildcard '*' is not allowed" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEndpointNormalize_BackendRequestParams(t *testing.T) {
	item := &Endpoint{
		Method: "GET",
		Path:   "doc",
		Backend: Backend{
			Headers: map[string]string{
				" X-Endpoint-Token ": " secret ",
				" ":                  "ignored",
			},
			QueryParams: map[string]string{
				" mode ": " full ",
				"":       "ignored",
			},
		},
	}

	if err := item.Normalize(); err != nil {
		t.Fatalf("Normalize() unexpected error: %v", err)
	}
	if item.Backend.Headers["X-Endpoint-Token"] != "secret" {
		t.Fatalf("header normalize failed: %#v", item.Backend.Headers)
	}
	if _, ok := item.Backend.Headers[""]; ok {
		t.Fatalf("empty header key was not removed: %#v", item.Backend.Headers)
	}
	if item.Backend.QueryParams["mode"] != "full" {
		t.Fatalf("query param normalize failed: %#v", item.Backend.QueryParams)
	}
}
