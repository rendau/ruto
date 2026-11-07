package oidc

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	authModel "github.com/rendau/ruto/internal/domain/auth/model"
)

// fakeProvider serves a discovery document naming issuer as itself.
func fakeProvider(t *testing.T, issuer func() string, token http.HandlerFunc) *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/realms/corp/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		base := issuer()
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer":                 base,
			"authorization_endpoint": base + "/protocol/openid-connect/auth",
			"token_endpoint":         base + "/protocol/openid-connect/token",
			"end_session_endpoint":   base + "/protocol/openid-connect/logout",
			"jwks_uri":               base + "/protocol/openid-connect/certs",
		})
	})
	if token != nil {
		mux.HandleFunc("/realms/corp/protocol/openid-connect/token", token)
	}
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

func TestMetadata(t *testing.T) {
	var issuer string
	server := fakeProvider(t, func() string { return issuer }, nil)
	issuer = server.URL + "/realms/corp"

	svc := New()
	meta, err := svc.Metadata(context.Background(), issuer, "")
	require.NoError(t, err)
	assert.Equal(t, issuer+"/protocol/openid-connect/auth", meta.AuthorizationEndpoint)
	assert.Equal(t, issuer+"/protocol/openid-connect/token", meta.TokenEndpoint)
	assert.Equal(t, issuer+"/protocol/openid-connect/certs", meta.JwksUri)
	assert.Equal(t, issuer+"/protocol/openid-connect/logout", meta.EndSessionEndpoint)

	// The second call is served from the cache: the server may go away.
	server.Close()
	again, err := svc.Metadata(context.Background(), issuer, "")
	require.NoError(t, err)
	assert.Same(t, meta, again)
}

func TestMetadataInternalUrl(t *testing.T) {
	// The document is read from the internal address but names the public
	// issuer; only the endpoints the gateway calls move to the internal one.
	const publicIssuer = "https://id.example.com/realms/corp"
	server := fakeProvider(t, func() string { return publicIssuer }, nil)
	internal := server.URL + "/realms/corp"

	meta, err := New().Metadata(context.Background(), publicIssuer, internal)
	require.NoError(t, err)
	assert.Equal(t, publicIssuer+"/protocol/openid-connect/auth", meta.AuthorizationEndpoint)
	assert.Equal(t, publicIssuer+"/protocol/openid-connect/logout", meta.EndSessionEndpoint)
	assert.Equal(t, internal+"/protocol/openid-connect/token", meta.TokenEndpoint)
	assert.Equal(t, internal+"/protocol/openid-connect/certs", meta.JwksUri)
}

func TestMetadataIssuerMismatch(t *testing.T) {
	server := fakeProvider(t, func() string { return "https://other.example.com/realms/x" }, nil)

	_, err := New().Metadata(context.Background(), server.URL+"/realms/corp", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not match")
}

func TestExchange(t *testing.T) {
	var issuer string
	var seen struct {
		user, pass string
		form       map[string]string
	}
	server := fakeProvider(t, func() string { return issuer }, func(w http.ResponseWriter, r *http.Request) {
		seen.user, seen.pass, _ = r.BasicAuth()
		require.NoError(t, r.ParseForm())
		seen.form = map[string]string{}
		for k := range r.PostForm {
			seen.form[k] = r.PostForm.Get(k)
		}
		if r.PostForm.Get("code") == "bad" {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid_grant", "error_description": "Code not valid"})
			return
		}
		_ = json.NewEncoder(w).Encode(TokenResponse{
			AccessToken: "at", RefreshToken: "rt", IdToken: "idt", ExpiresIn: 300, RefreshExpiresIn: 1800,
		})
	})
	issuer = server.URL + "/realms/corp"

	svc := New()
	meta, err := svc.Metadata(context.Background(), issuer, "")
	require.NoError(t, err)

	client := &authModel.JwtClient{Id: "ruto", Secret: "s3cret"}
	rep, err := svc.Exchange(context.Background(), meta.TokenEndpoint, client, "the-code", "verifier", "https://gw/_auth/corp/callback")
	require.NoError(t, err)
	assert.Equal(t, "at", rep.AccessToken)
	assert.Equal(t, 1800, rep.RefreshExpiresIn)
	assert.Equal(t, "ruto", seen.user)
	assert.Equal(t, "s3cret", seen.pass)
	assert.Equal(t, map[string]string{
		"grant_type":    "authorization_code",
		"code":          "the-code",
		"code_verifier": "verifier",
		"redirect_uri":  "https://gw/_auth/corp/callback",
	}, seen.form)

	_, err = svc.Exchange(context.Background(), meta.TokenEndpoint, client, "bad", "verifier", "https://gw/_auth/corp/callback")
	var refusal *Error
	require.ErrorAs(t, err, &refusal)
	assert.Equal(t, "invalid_grant", refusal.Code)
	assert.Equal(t, http.StatusBadRequest, refusal.Status)

	_, err = svc.Refresh(context.Background(), meta.TokenEndpoint, client, "rt")
	require.NoError(t, err)
	assert.Equal(t, "refresh_token", seen.form["grant_type"])
	assert.Equal(t, "rt", seen.form["refresh_token"])
}
