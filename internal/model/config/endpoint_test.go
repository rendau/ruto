package config

import (
	"errors"
	"reflect"
	"testing"
)

func TestEndpointNormalize(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := Endpoint{
			Id:     "  endpoint-id  ",
			Method: " post ",
			Path:   " /v1/items ",
			Backend: EndpointBackend{
				Path: " /api/items ",
			},
			JwtValidation: EndpointJwtValidation{
				Roles: []string{" admin ", "", " user "},
			},
			IpValidation: EndpointIpValidation{
				AllowedIps: []string{" 127.0.0.1 ", "", "10.0.0.1"},
			},
		}

		err := m.Normalize()
		if err != nil {
			t.Fatalf("Normalize() error = %v", err)
		}

		if m.Id != "endpoint-id" {
			t.Fatalf("Id = %q, want %q", m.Id, "endpoint-id")
		}
		if m.Method != "POST" {
			t.Fatalf("Method = %q, want %q", m.Method, "POST")
		}
		if m.Path != "v1/items" {
			t.Fatalf("Path = %q, want %q", m.Path, "v1/items")
		}
		if m.Backend.Path != "api/items" {
			t.Fatalf("Backend.Path = %q, want %q", m.Backend.Path, "api/items")
		}
		if !reflect.DeepEqual(m.JwtValidation.Roles, []string{"admin", "user"}) {
			t.Fatalf("JwtValidation.Roles = %#v, want %#v", m.JwtValidation.Roles, []string{"admin", "user"})
		}
		if !reflect.DeepEqual(m.IpValidation.AllowedIps, []string{"127.0.0.1", "10.0.0.1"}) {
			t.Fatalf("IpValidation.AllowedIps = %#v, want %#v", m.IpValidation.AllowedIps, []string{"127.0.0.1", "10.0.0.1"})
		}
	})

	t.Run("error empty method", func(t *testing.T) {
		m := Endpoint{
			Method: "   ",
			Path:   "/ok",
			Backend: EndpointBackend{
				Path: "/ok",
			},
		}

		err := m.Normalize()
		if !errors.Is(err, errEmptyValue) {
			t.Fatalf("Normalize() error = %v, want errEmptyValue", err)
		}
	})
}

func TestEndpointBackendNormalize(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := EndpointBackend{Path: " /backend/path "}
		err := m.Normalize()
		if err != nil {
			t.Fatalf("Normalize() error = %v", err)
		}
		if m.Path != "backend/path" {
			t.Fatalf("Path = %q, want %q", m.Path, "backend/path")
		}
	})

	t.Run("error empty path", func(t *testing.T) {
		m := EndpointBackend{Path: " / "}
		err := m.Normalize()
		if !errors.Is(err, errEmptyValue) {
			t.Fatalf("Normalize() error = %v, want errEmptyValue", err)
		}
	})
}

func TestEndpointJwtValidationNormalize(t *testing.T) {
	m := EndpointJwtValidation{Roles: []string{" role1 ", "", "role2 "}}
	err := m.Normalize()
	if err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}
	if !reflect.DeepEqual(m.Roles, []string{"role1", "role2"}) {
		t.Fatalf("Roles = %#v, want %#v", m.Roles, []string{"role1", "role2"})
	}
}

func TestEndpointIpValidationNormalize(t *testing.T) {
	m := EndpointIpValidation{AllowedIps: []string{" 1.1.1.1 ", "", "2.2.2.2 "}}
	err := m.Normalize()
	if err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}
	if !reflect.DeepEqual(m.AllowedIps, []string{"1.1.1.1", "2.2.2.2"}) {
		t.Fatalf("AllowedIps = %#v, want %#v", m.AllowedIps, []string{"1.1.1.1", "2.2.2.2"})
	}
}
