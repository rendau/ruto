package oidc

// Metadata is the part of an OpenID Connect discovery document the gateway
// uses. JwksUri is the address the gateway itself calls, already pointed at
// the provider's internal address when one is configured.
type Metadata struct {
	Issuer  string `json:"issuer"`
	JwksUri string `json:"jwks_uri"`
}
