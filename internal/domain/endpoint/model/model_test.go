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
