package config

import (
	"strings"
	"testing"
	"time"
)

func TestEndpointNormalize(t *testing.T) {
	m := Endpoint{
		Id:     "  users.list  ",
		Method: "  get  ",
		Path:   " /v1/users ",
		Backend: EndpointBackend{
			Path: " /internal/users ",
		},
		JwtValidation: EndpointJwtValidation{
			Roles: []string{" admin ", "", " user ", "   "},
		},
		IpValidation: EndpointIpValidation{
			AllowedIps: []string{" 10.0.0.1 ", "", " 127.0.0.1 "},
		},
	}

	m.Normalize()

	if m.Id != "users.list" {
		t.Fatalf("unexpected Id: %q", m.Id)
	}
	if m.Method != "GET" {
		t.Fatalf("unexpected Method: %q", m.Method)
	}
	if m.Path != "v1/users" {
		t.Fatalf("unexpected Path: %q", m.Path)
	}
	if m.Backend.Path != "internal/users" {
		t.Fatalf("unexpected Backend.Path: %q", m.Backend.Path)
	}
	if got, want := strings.Join(m.JwtValidation.Roles, ","), "admin,user"; got != want {
		t.Fatalf("unexpected Roles: %q", got)
	}
	if got, want := strings.Join(m.IpValidation.AllowedIps, ","), "10.0.0.1,127.0.0.1"; got != want {
		t.Fatalf("unexpected AllowedIps: %q", got)
	}
}

func TestEndpointValidate(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		m := Endpoint{
			Id:     "id",
			Method: "GET",
			Path:   "v1",
			Backend: EndpointBackend{
				Path: "backend",
			},
		}
		if err := m.Validate(); err != nil {
			t.Fatalf("Validate() error = %v", err)
		}
	})

	t.Run("wraps backend path error", func(t *testing.T) {
		m := Endpoint{
			Id:     "id",
			Method: "GET",
			Path:   "v1",
			Backend: EndpointBackend{
				Path: "",
			},
		}
		err := m.Validate()
		if err == nil {
			t.Fatal("expected error")
		}
		if got, want := err.Error(), "backend: path: must not be empty"; got != want {
			t.Fatalf("unexpected error: %q", got)
		}
	})
}

func TestAppNormalizeAndValidate(t *testing.T) {
	m := App{
		Id:         "  app1  ",
		PublicPath: " /api ",
		Backend: AppBackend{
			Host: "  http://backend.local  ",
			Path: " /svc ",
		},
		Endpoints: []Endpoint{
			{
				Id:     "  ep1 ",
				Method: " post ",
				Path:   " /users ",
				Backend: EndpointBackend{
					Path: " /users ",
				},
			},
		},
	}

	m.Normalize()

	if m.Id != "app1" {
		t.Fatalf("unexpected Id: %q", m.Id)
	}
	if m.PublicPath != "api" {
		t.Fatalf("unexpected PublicPath: %q", m.PublicPath)
	}
	if m.Backend.Host != "http://backend.local" {
		t.Fatalf("unexpected Backend.Host: %q", m.Backend.Host)
	}
	if m.Backend.Path != "svc" {
		t.Fatalf("unexpected Backend.Path: %q", m.Backend.Path)
	}
	if m.Endpoints[0].Method != "POST" {
		t.Fatalf("unexpected nested endpoint method: %q", m.Endpoints[0].Method)
	}

	if err := m.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestAppValidateWrapsNestedEndpointError(t *testing.T) {
	m := App{
		Id:         "app",
		PublicPath: "api",
		Backend: AppBackend{
			Host: "http://backend",
			Path: "svc",
		},
		Endpoints: []Endpoint{
			{
				Id:   "",
				Path: "x",
				Backend: EndpointBackend{
					Path: "b",
				},
			},
		},
	}

	err := m.Validate()
	if err == nil {
		t.Fatal("expected error")
	}
	if got, want := err.Error(), "endpoints[0]: id: must not be empty"; got != want {
		t.Fatalf("unexpected error: %q", got)
	}
}

func TestRootNormalizeAndValidate(t *testing.T) {
	m := Root{
		PublicBaseUrl: " https://example.com/ ",
		Timeout: RootTimeout{
			Global:     time.Second,
			ReadHeader: 2 * time.Second,
			Read:       3 * time.Second,
		},
		Cors: RootCors{
			MaxAge:       " 1h ",
			AllowOrigins: []string{" https://a.com ", "", " https://b.com "},
			AllowMethods: []string{" GET ", " ", " POST "},
			AllowHeaders: []string{" Authorization ", "", " Content-Type "},
		},
		Jwt: []RootJwt{
			{
				JwkUrl:        " https://issuer/.well-known/jwks.json ",
				Alg:           " rs256 ",
				CacheDuration: time.Minute,
				RolesPath:     " realm_access.roles ",
			},
		},
		Apps: []App{
			{
				Id:         " app1 ",
				PublicPath: " /api ",
				Backend: AppBackend{
					Host: " http://backend ",
					Path: " /svc ",
				},
			},
		},
	}

	m.Normalize()

	if m.PublicBaseUrl != "https://example.com" {
		t.Fatalf("unexpected PublicBaseUrl: %q", m.PublicBaseUrl)
	}
	if m.Cors.MaxAge != "1h" {
		t.Fatalf("unexpected Cors.MaxAge: %q", m.Cors.MaxAge)
	}
	if got, want := strings.Join(m.Cors.AllowOrigins, ","), "https://a.com,https://b.com"; got != want {
		t.Fatalf("unexpected AllowOrigins: %q", got)
	}
	if got, want := strings.Join(m.Cors.AllowMethods, ","), "GET,POST"; got != want {
		t.Fatalf("unexpected AllowMethods: %q", got)
	}
	if m.Jwt[0].Alg != "RS256" {
		t.Fatalf("unexpected Jwt.Alg: %q", m.Jwt[0].Alg)
	}

	if err := m.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestRootValidateWrapsNestedError(t *testing.T) {
	m := Root{
		PublicBaseUrl: "https://example.com",
		Timeout: RootTimeout{
			Global: -1 * time.Second,
		},
	}

	err := m.Validate()
	if err == nil {
		t.Fatal("expected error")
	}
	if got, want := err.Error(), "timeout: global: must be >= 0"; got != want {
		t.Fatalf("unexpected error: %q", got)
	}
}
