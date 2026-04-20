package config

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestRootNormalize(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := Root{
			PublicBaseUrl: " https://public.example.com/ ",
			Timeout: RootTimeout{
				Global:     5 * time.Second,
				ReadHeader: 2 * time.Second,
				Read:       3 * time.Second,
			},
			Cors: RootCors{
				MaxAge:       "  1h ",
				AllowOrigins: []string{" https://a.example ", "", "https://b.example"},
				AllowMethods: []string{" GET ", "", "POST"},
				AllowHeaders: []string{" Authorization ", "", "Content-Type"},
			},
			Jwt: []*RootJwt{
				{
					JwkUrl:        " https://issuer.example/.well-known/jwks.json ",
					Alg:           " rs256 ",
					CacheDuration: 1 * time.Minute,
					RolesPath:     " roles.path ",
				},
			},
			Apps: []*App{
				{
					PublicPathPrefix: " /api ",
					Backend:          AppBackend{UrlStr: "https://backend.example"},
				},
			},
		}

		err := m.Normalize()
		if err != nil {
			t.Fatalf("Normalize() error = %v", err)
		}

		if m.PublicBaseUrl != "https://public.example.com" {
			t.Fatalf("PublicBaseUrl = %q, want %q", m.PublicBaseUrl, "https://public.example.com")
		}
		if m.Jwt[0].Alg != "RS256" {
			t.Fatalf("Jwt[0].Alg = %q, want %q", m.Jwt[0].Alg, "RS256")
		}
		if !reflect.DeepEqual(m.Cors.AllowMethods, []string{"GET", "POST"}) {
			t.Fatalf("Cors.AllowMethods = %#v, want %#v", m.Cors.AllowMethods, []string{"GET", "POST"})
		}
	})

	t.Run("error empty public base url", func(t *testing.T) {
		m := Root{}
		err := m.Normalize()
		if !errors.Is(err, errEmptyValue) {
			t.Fatalf("Normalize() error = %v, want errEmptyValue", err)
		}
	})

	t.Run("error nested app normalize", func(t *testing.T) {
		m := Root{
			PublicBaseUrl: "https://public.example.com",
			Jwt: []*RootJwt{
				{
					JwkUrl:        "https://issuer.example/jwks",
					Alg:           "RS256",
					CacheDuration: time.Second,
					RolesPath:     "roles",
				},
			},
			Apps: []*App{
				{
					PublicPathPrefix: "api",
					Backend:          AppBackend{UrlStr: "://bad"},
				},
			},
		}
		err := m.Normalize()
		if err == nil {
			t.Fatal("Normalize() error = nil, want non-nil")
		}
	})
}

func TestRootTimeoutNormalize(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := RootTimeout{
			Global:     1 * time.Second,
			ReadHeader: 2 * time.Second,
			Read:       3 * time.Second,
		}
		err := m.Normalize()
		if err != nil {
			t.Fatalf("Normalize() error = %v", err)
		}
	})

	t.Run("error negative value", func(t *testing.T) {
		m := RootTimeout{Read: -1}
		err := m.Normalize()
		if !errors.Is(err, errNegativeValue) {
			t.Fatalf("Normalize() error = %v, want errNegativeValue", err)
		}
	})
}

func TestRootCorsNormalize(t *testing.T) {
	m := RootCors{
		MaxAge:       " 30m ",
		AllowOrigins: []string{" https://a.example ", "", "https://b.example"},
		AllowMethods: []string{" GET ", "", "POST "},
		AllowHeaders: []string{" Authorization ", "", "Content-Type "},
	}

	err := m.Normalize()
	if err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}

	if m.MaxAge != "30m" {
		t.Fatalf("MaxAge = %q, want %q", m.MaxAge, "30m")
	}
	if !reflect.DeepEqual(m.AllowOrigins, []string{"https://a.example", "https://b.example"}) {
		t.Fatalf("AllowOrigins = %#v, want %#v", m.AllowOrigins, []string{"https://a.example", "https://b.example"})
	}
	if !reflect.DeepEqual(m.AllowMethods, []string{"GET", "POST"}) {
		t.Fatalf("AllowMethods = %#v, want %#v", m.AllowMethods, []string{"GET", "POST"})
	}
	if !reflect.DeepEqual(m.AllowHeaders, []string{"Authorization", "Content-Type"}) {
		t.Fatalf("AllowHeaders = %#v, want %#v", m.AllowHeaders, []string{"Authorization", "Content-Type"})
	}
}

func TestRootJwtNormalize(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		m := RootJwt{
			JwkUrl:        " https://issuer.example/jwks ",
			Alg:           " rs256 ",
			CacheDuration: 5 * time.Minute,
			RolesPath:     " roles ",
		}
		err := m.Normalize()
		if err != nil {
			t.Fatalf("Normalize() error = %v", err)
		}
		if m.JwkUrl != "https://issuer.example/jwks" {
			t.Fatalf("JwkUrl = %q, want %q", m.JwkUrl, "https://issuer.example/jwks")
		}
		if m.Alg != "RS256" {
			t.Fatalf("Alg = %q, want %q", m.Alg, "RS256")
		}
		if m.RolesPath != "roles" {
			t.Fatalf("RolesPath = %q, want %q", m.RolesPath, "roles")
		}
	})

	t.Run("error empty roles path", func(t *testing.T) {
		m := RootJwt{
			JwkUrl:        "https://issuer.example/jwks",
			Alg:           "RS256",
			CacheDuration: time.Minute,
			RolesPath:     " ",
		}
		err := m.Normalize()
		if !errors.Is(err, errEmptyValue) {
			t.Fatalf("Normalize() error = %v, want errEmptyValue", err)
		}
	})
}
