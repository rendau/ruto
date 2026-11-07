package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	authModel "github.com/rendau/ruto/internal/domain/auth/model"
	endpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
	varsModel "github.com/rendau/ruto/internal/domain/vars/model"
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

func TestAppNormalize_WildcardEndpointNeedsAllowWildcard(t *testing.T) {
	newApp := func(allow bool) *App {
		return &App{
			PathPrefix:    "account",
			AllowWildcard: allow,
			Backend:       Backend{Url: "http://example.local/svc"},
			Endpoints: []*endpointModel.Endpoint{
				{Type: endpointModel.TypeHTTP, Http: endpointModel.Http{Method: "GET", Path: "profile"}},
				{Type: endpointModel.TypeHTTP, Http: endpointModel.Http{Method: "GET", Path: "docs/*"}},
			},
		}
	}

	require.EqualError(t, newApp(false).Normalize(), "endpoints[1]: http: path: wildcard '*' is not allowed for this app")
	require.NoError(t, newApp(true).Normalize())
}

func TestAppNormalize_RejectReservedAuthPrefix(t *testing.T) {
	for _, prefix := range []string{"_auth", "/_auth/", "_auth/corp"} {
		item := &App{
			PathPrefix: prefix,
			Backend:    Backend{Url: "http://example.local/svc"},
		}
		require.EqualError(t, item.Normalize(), "path_prefix: /_auth is reserved for the gateway login routes", prefix)
	}

	item := &App{
		PathPrefix: "_authors",
		Backend:    Backend{Url: "http://example.local/svc"},
	}
	require.NoError(t, item.Normalize())
}

func TestAppNormalize_Jwt(t *testing.T) {
	item := &App{
		PathPrefix: "shop",
		Backend:    Backend{Url: "http://example.local/svc"},
		Jwt: []AppJwt{
			{Provider: " corp ", Client: authModel.JwtClient{Id: "shop", Secret: "s", Scopes: []string{"email"}}},
		},
	}
	require.NoError(t, item.Normalize())
	assert.Equal(t, "corp", item.Jwt[0].Provider)
	assert.Equal(t, []string{"openid", "email"}, item.Jwt[0].Client.Scopes)
	assert.Equal(t, "ruto_corp_shop", item.JwtCookieName("corp"))
	assert.Equal(t, "ruto_other", item.JwtCookieName("other"))

	item.Jwt = append(item.Jwt, AppJwt{Provider: "corp", Client: authModel.JwtClient{Id: "x", Secret: "y"}})
	require.EqualError(t, item.Normalize(), `jwt[1]: provider: duplicate "corp"`)

	item.Jwt = []AppJwt{{Provider: "corp", Client: authModel.JwtClient{Id: "shop"}}}
	require.EqualError(t, item.Normalize(), "jwt[0]: client: secret: empty")

	item.Jwt = []AppJwt{{Client: authModel.JwtClient{Id: "shop", Secret: "s"}}}
	require.EqualError(t, item.Normalize(), "jwt[0]: provider: empty")

	item.Jwt = nil
	require.NoError(t, item.Normalize())
	assert.NotNil(t, item.Jwt)
}

func TestAppRedacted_JwtClientSecret(t *testing.T) {
	item := &App{
		PathPrefix: "shop",
		Backend:    Backend{Url: "http://example.local/svc"},
		Jwt:        []AppJwt{{Provider: "corp", Client: authModel.JwtClient{Id: "shop", Secret: "s3cret", Scopes: []string{"openid"}}}},
	}
	redacted := item.Redacted()
	assert.Equal(t, varsModel.RedactedPlaceholder, redacted.Jwt[0].Client.Secret)
	assert.Equal(t, "shop", redacted.Jwt[0].Client.Id)
	assert.Equal(t, "s3cret", item.Jwt[0].Client.Secret, "the receiver is left untouched")
}
