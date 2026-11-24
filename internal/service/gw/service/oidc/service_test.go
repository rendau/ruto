package oidc

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeProvider serves a discovery document naming issuer as itself.
func fakeProvider(t *testing.T, issuer func() string) *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/realms/corp/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		base := issuer()
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer":   base,
			"jwks_uri": base + "/protocol/openid-connect/certs",
		})
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

func TestMetadata(t *testing.T) {
	var issuer string
	server := fakeProvider(t, func() string { return issuer })
	issuer = server.URL + "/realms/corp"

	svc := New()
	meta, err := svc.Metadata(context.Background(), issuer, "")
	require.NoError(t, err)
	assert.Equal(t, issuer+"/protocol/openid-connect/certs", meta.JwksUri)

	// The second call is served from the cache: the server may go away.
	server.Close()
	again, err := svc.Metadata(context.Background(), issuer, "")
	require.NoError(t, err)
	assert.Same(t, meta, again)
}

func TestForget(t *testing.T) {
	var issuer string
	server := fakeProvider(t, func() string { return issuer })
	issuer = server.URL + "/realms/corp"

	svc := New()
	meta, err := svc.Metadata(context.Background(), issuer, "")
	require.NoError(t, err)

	svc.Forget(issuer, "")
	again, err := svc.Metadata(context.Background(), issuer, "")
	require.NoError(t, err)
	assert.NotSame(t, meta, again, "a forgotten document is read again")
}

func TestMetadataInternalUrl(t *testing.T) {
	// The document is read from the internal address but names the public
	// issuer; jwks_uri moves to the internal one.
	const publicIssuer = "https://id.example.com/realms/corp"
	server := fakeProvider(t, func() string { return publicIssuer })
	internal := server.URL + "/realms/corp"

	meta, err := New().Metadata(context.Background(), publicIssuer, internal)
	require.NoError(t, err)
	assert.Equal(t, internal+"/protocol/openid-connect/certs", meta.JwksUri)
}

func TestMetadataIssuerMismatch(t *testing.T) {
	server := fakeProvider(t, func() string { return "https://other.example.com/realms/x" })

	_, err := New().Metadata(context.Background(), server.URL+"/realms/corp", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not match")
}
