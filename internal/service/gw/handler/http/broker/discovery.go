package broker

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/rendau/ruto/internal/service/gw/service/oidc"
)

// cacheControl lets a front's SDK keep the discovery document and the keys
// for a while; a new signing key reaches the fronts within it.
const cacheControl = "public, max-age=300"

// brokerEndpoints are the endpoints of the provider's document the broker
// answers itself, under its own address.
var brokerEndpoints = []string{
	"authorization_endpoint",
	"token_endpoint",
	"userinfo_endpoint",
	"end_session_endpoint",
}

// copiedMembers are taken from the provider's document as they are: they
// describe the user and the provider's pages, which the broker passes on. The
// rest of the document (introspection, registration, device flow, request
// objects, client authentication by keys…) describes what the broker does not
// do, so it is left out rather than contradicted.
var copiedMembers = []string{
	"scopes_supported",
	"claims_supported",
	"claim_types_supported",
	"subject_types_supported",
	"acr_values_supported",
	"display_values_supported",
	"ui_locales_supported",
	"claims_locales_supported",
	"service_documentation",
	"op_policy_uri",
	"op_tos_uri",
}

// discovery answers with the provider's document made the broker's for one
// front: its own issuer, the provider's endpoint paths under its address, and
// what the broker supports: the authorization code with PKCE for a public
// client, ruto's ES256 ID tokens.
func (u *unit) discovery(w http.ResponseWriter, meta *oidc.Metadata, client string) {
	front := u.publicUrl + u.basePath + "/" + client

	doc := make(map[string]any, len(copiedMembers)+16)
	for _, name := range copiedMembers {
		if value, ok := meta.Document[name]; ok {
			doc[name] = value
		}
	}
	if _, ok := doc["subject_types_supported"]; !ok {
		doc["subject_types_supported"] = []string{"public"} // required by the standard
	}

	doc["issuer"] = front + u.issuerPath
	for _, name := range append([]string{"jwks_uri"}, brokerEndpoints...) {
		if p := endpointPath(meta.PublicEndpoint(name)); p != "" {
			doc[name] = front + p
		}
	}
	doc["response_types_supported"] = []string{"code"}
	doc["response_modes_supported"] = []string{"query", "fragment"}
	doc["grant_types_supported"] = []string{"authorization_code", "refresh_token"}
	doc["code_challenge_methods_supported"] = []string{"S256"}
	doc["token_endpoint_auth_methods_supported"] = []string{"none"}
	doc["id_token_signing_alg_values_supported"] = []string{"ES256"}
	// Its default is true: a provider that says nothing supports request_uri.
	doc["request_uri_parameter_supported"] = false

	body, err := json.Marshal(doc)
	if err != nil {
		slog.Error("broker: discovery", "provider", u.name, "error", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	writeJSON(w, body)
}

// jwks answers with ruto's public keys only: fronts check ruto's tokens by
// them, the provider's tokens are none of their business.
func (u *unit) jwks(w http.ResponseWriter) {
	body, ok := u.deps.Keys.JWKS()
	if !ok {
		http.Error(w, "no signing key", http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, body)
}

func writeJSON(w http.ResponseWriter, body []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", cacheControl)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}
