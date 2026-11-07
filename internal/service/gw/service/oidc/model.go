package oidc

import "fmt"

// Metadata is the part of an OpenID Connect discovery document the gateway
// uses. TokenEndpoint and JwksUri are the addresses the gateway itself calls,
// already pointed at the provider's internal address when one is configured;
// the browser-facing endpoints stay public.
type Metadata struct {
	Issuer                string `json:"issuer"`
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	EndSessionEndpoint    string `json:"end_session_endpoint"`
	JwksUri               string `json:"jwks_uri"`
}

// TokenResponse is the token endpoint's answer to a code exchange or a
// refresh. RefreshExpiresIn is a common extension (Keycloak), zero when the
// provider does not send it.
type TokenResponse struct {
	AccessToken      string `json:"access_token"`
	TokenType        string `json:"token_type"`
	ExpiresIn        int    `json:"expires_in"`
	RefreshToken     string `json:"refresh_token"`
	RefreshExpiresIn int    `json:"refresh_expires_in"`
	IdToken          string `json:"id_token"`
	Scope            string `json:"scope"`
}

// Error is the provider's refusal (RFC 6749 §5.2): the code is what the
// provider said, Status the http status it said it with.
type Error struct {
	Status      int
	Code        string `json:"error"`
	Description string `json:"error_description"`
}

func (e *Error) Error() string {
	if e.Description != "" {
		return fmt.Sprintf("provider refused (%d): %s: %s", e.Status, e.Code, e.Description)
	}
	return fmt.Sprintf("provider refused (%d): %s", e.Status, e.Code)
}
