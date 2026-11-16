package broker

import (
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/rendau/ruto/internal/service/gw/handler/http/authflow"
)

// authError is the parameter a failed login at the provider comes back to the
// authorization endpoint with (see authflow.LoginPath).
const authError = "auth_error"

// promptValues are the values OpenID Connect Core §3.1.2.1 defines for prompt.
var promptValues = map[string]bool{"none": true, "login": true, "consent": true, "select_account": true}

// authorize is the authorization endpoint (OpenID Connect Core §3.1.2): the
// code flow with PKCE S256 for a public client. With a live login session on
// the gateway the front gets a code at once; without one the person logs in
// at the provider through the gateway's own login, which comes back here.
func (u *unit) authorize(w http.ResponseWriter, r *http.Request, client string) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		methodNotAllowed(w, "GET, POST")
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "the request is not readable", http.StatusBadRequest)
		return
	}
	params := r.Form

	// Without a valid client and return address there is no one to answer:
	// the error is shown here (RFC 6749 §4.1.2.1).
	if params.Get("client_id") != client {
		http.Error(w, "client_id must be the client of this address", http.StatusBadRequest)
		return
	}
	redirectUri := params.Get("redirect_uri")
	if !u.redirectAllowed(redirectUri) {
		http.Error(w, "redirect_uri is not allowed: an absolute address on a CORS origin is required", http.StatusBadRequest)
		return
	}

	reply := &authReply{redirectUri: redirectUri, state: params.Get("state"), issuer: u.frontIssuer(client), mode: "query"}
	switch mode := params.Get("response_mode"); mode {
	case "", "query":
	case "fragment":
		reply.mode = mode
	default:
		reply.fail(w, r, "invalid_request", "response_mode must be query or fragment")
		return
	}

	if code := params.Get(authError); code != "" {
		reply.fail(w, r, errorCode(code), "login at the provider failed")
		return
	}
	if params.Get("response_type") != "code" {
		reply.fail(w, r, "unsupported_response_type", "only the code flow is supported")
		return
	}
	challenge := params.Get("code_challenge")
	if params.Get("code_challenge_method") != "S256" || !validChallenge(challenge) {
		reply.fail(w, r, "invalid_request", "PKCE with code_challenge_method=S256 is required")
		return
	}
	prompt, ok := parsePrompt(params.Get("prompt"))
	if !ok {
		reply.fail(w, r, "invalid_request", "prompt is not valid")
		return
	}

	if slices.Contains(strings.Fields(prompt), "login") {
		u.toLogin(w, r, params, prompt)
		return
	}

	session, err := u.sessions.Session(w, r)
	switch {
	case err == nil:
	case errors.Is(err, authflow.ErrNoSession):
		if prompt == "none" {
			reply.fail(w, r, "login_required", "no login session")
			return
		}
		u.toLogin(w, r, params, prompt)
		return
	default:
		slog.Warn("broker: session is unavailable", "provider", u.name, "client", client, "error", err)
		reply.fail(w, r, "temporarily_unavailable", "login provider is unavailable")
		return
	}

	sub, _ := session.Claims["sub"].(string)
	if sub == "" {
		slog.Warn("broker: the shared token has no sub", "provider", u.name)
		reply.fail(w, r, "server_error", "the session has no subject")
		return
	}
	code, ok := u.issueCode(client, &loginCode{
		RedirectUri: redirectUri,
		Challenge:   challenge,
		Nonce:       params.Get("nonce"),
		Scope:       params.Get("scope"),
		Sub:         sub,
		AuthTime:    session.AuthTime,
	})
	if !ok {
		slog.Error("broker: no cookie key, login codes cannot be issued", "provider", u.name)
		reply.fail(w, r, "server_error", "the gateway cannot issue login codes")
		return
	}
	reply.succeed(w, r, code)
}

// toLogin sends the browser to the gateway's login at the provider, to come
// back to this very request: without prompt (it is passed to the provider and
// done with) and without a previous login error.
func (u *unit) toLogin(w http.ResponseWriter, r *http.Request, params url.Values, prompt string) {
	again := url.Values{}
	for key, values := range params {
		if key != "prompt" && key != authError {
			again[key] = values
		}
	}
	returnPath := r.URL.EscapedPath() + "?" + again.Encode()
	http.Redirect(w, r, u.sessions.LoginPath(returnPath, prompt), http.StatusFound)
}

// redirectAllowed checks a front's return address: an absolute http(s) url
// without a fragment, on the gateway's origin or an origin the CORS settings
// name ("*" does not count, it would be an open redirect).
func (u *unit) redirectAllowed(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Fragment != "" || strings.Contains(raw, "#") {
		return false
	}
	origin := originOf(raw)
	if origin == "" {
		return false
	}
	return origin == u.publicOrigin || u.origins.Listed(origin)
}

// authReply answers a front at its return address (RFC 6749 §4.1.2), with
// the issuer of the answer (RFC 9207).
type authReply struct {
	redirectUri string
	state       string
	issuer      string
	mode        string // query or fragment
}

func (a *authReply) succeed(w http.ResponseWriter, r *http.Request, code string) {
	a.send(w, r, url.Values{"code": {code}})
}

func (a *authReply) fail(w http.ResponseWriter, r *http.Request, code, description string) {
	a.send(w, r, url.Values{"error": {code}, "error_description": {description}})
}

func (a *authReply) send(w http.ResponseWriter, r *http.Request, values url.Values) {
	if a.state != "" {
		values.Set("state", a.state)
	}
	values.Set("iss", a.issuer)
	if a.mode == "fragment" {
		// the return address has no fragment (redirectAllowed)
		http.Redirect(w, r, a.redirectUri+"#"+values.Encode(), http.StatusFound)
		return
	}
	target, _ := url.Parse(a.redirectUri) // checked by redirectAllowed
	query := target.Query()
	for key, v := range values {
		query[key] = v
	}
	target.RawQuery = query.Encode()
	http.Redirect(w, r, target.String(), http.StatusFound)
}

// validChallenge: an S256 challenge is base64url of a sha256, 43 characters.
func validChallenge(challenge string) bool {
	if len(challenge) != 43 {
		return false
	}
	for _, c := range challenge {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_') {
			return false
		}
	}
	return true
}

// parsePrompt validates prompt: standard values separated by spaces, "none"
// only alone. Empty means no prompt.
func parsePrompt(raw string) (string, bool) {
	values := strings.Fields(raw)
	for _, value := range values {
		if !promptValues[value] || (value == "none" && len(values) > 1) {
			return "", false
		}
	}
	return strings.Join(values, " "), true
}

// errorCode keeps an error code as it is when it looks like one and hides
// anything else behind server_error: the value ends up in a front's address.
func errorCode(raw string) string {
	if raw == "" || len(raw) > 64 {
		return "server_error"
	}
	for _, c := range raw {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '-' || c == '.') {
			return "server_error"
		}
	}
	return raw
}
