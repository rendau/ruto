package broker

import (
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/rendau/ruto/internal/constant"
	"github.com/rendau/ruto/internal/service/gw/handler/http/authflow"
)

// promptValues are the values OpenID Connect Core §3.1.2.1 defines for prompt.
var promptValues = map[string]bool{"none": true, "login": true, "consent": true, "select_account": true}

// authorize is the authorization endpoint (OpenID Connect Core §3.1.2): the
// code flow with PKCE S256 for a public client. With a live login session on
// the gateway the front gets a code at once; without one the person logs in
// at the provider with the gateway's own client, and the callback comes back
// here.
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
	maxAge, ok := parseMaxAge(params.Get("max_age"))
	if !ok {
		reply.fail(w, r, "invalid_request", "max_age must be a non-negative number of seconds")
		return
	}

	if slices.Contains(strings.Fields(prompt), "login") {
		u.toLogin(w, r, reply, params, prompt)
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
		u.toLogin(w, r, reply, params, prompt)
		return
	default:
		slog.Warn("broker: session is unavailable", "provider", u.name, "client", client, "error", err)
		reply.fail(w, r, "temporarily_unavailable", "login provider is unavailable")
		return
	}

	// max_age: a person who entered the credentials longer ago, or at a time
	// the provider did not say, logs in again (OpenID Connect Core §3.1.2.1).
	if maxAge >= 0 && (session.AuthTime == 0 || time.Now().Unix()-session.AuthTime > maxAge) {
		if prompt == "none" {
			reply.fail(w, r, "login_required", "the login is older than max_age")
			return
		}
		u.toLogin(w, r, reply, params, prompt)
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

// toLogin starts the gateway's login at the provider, to come back to this
// very request: without prompt and max_age, which the provider gets and is
// done with (they would send the person to the provider again and again).
func (u *unit) toLogin(w http.ResponseWriter, r *http.Request, reply *authReply, params url.Values, prompt string) {
	again := url.Values{}
	for key, values := range params {
		if key != "prompt" && key != "max_age" {
			again[key] = values
		}
	}
	ret := r.URL.EscapedPath() + "?" + again.Encode()

	opts := &authflow.LoginOptions{
		Prompt:    prompt,
		MaxAge:    params.Get("max_age"),
		LoginHint: params.Get("login_hint"),
		UiLocales: params.Get("ui_locales"),
		AcrValues: params.Get("acr_values"),
	}
	if err := u.sessions.Login(w, r, ret, opts); err != nil {
		slog.Warn("broker: login at the provider cannot start", "provider", u.name, "error", err)
		reply.fail(w, r, "temporarily_unavailable", "login provider is unavailable")
	}
}

// callback is the provider's return address of the gateway's login
// (/_auth/<provider>/callback, the one the gateway's client has at the
// provider). A login that went through goes back to the authorization request
// that started it, which now finds the session; a failed one is answered at
// the front's return address with the standard error. Without a known
// request there is no one to answer: the error is shown here.
func (u *unit) callback(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	ret, err := u.sessions.Callback(w, r)
	reply := u.replyOf(ret)

	if err == nil {
		if reply == nil {
			http.Error(w, "the login went through, but the request it was started for is not valid", http.StatusBadRequest)
			return
		}
		http.Redirect(w, r, u.publicUrl+ret, http.StatusSeeOther)
		return
	}

	failure, ok := errors.AsType[*authflow.LoginError](err)
	if !ok {
		failure = &authflow.LoginError{Code: "server_error", Status: http.StatusInternalServerError, Text: "login failed"}
	}
	if reply == nil {
		http.Error(w, failure.Text, failure.Status)
		return
	}
	reply.fail(w, r, failure.Code, failure.Text)
}

// replyOf rebuilds the answer to the front from the authorization request a
// login was started for (toLogin) and checks it again: the login cookie came
// from the browser. nil for anything that is not a request to this
// provider's broker with an allowed return address.
func (u *unit) replyOf(ret string) *authReply {
	if ret == "" {
		return nil
	}
	target, err := url.Parse(ret)
	if err != nil || target.Host != "" || target.Scheme != "" {
		return nil
	}
	params := target.Query()
	client := params.Get("client_id")
	if !constant.BrokerClientIsValid(client) || !strings.HasPrefix(target.Path, constant.BrokerPath(u.name, client)+"/") {
		return nil
	}
	redirectUri := params.Get("redirect_uri")
	if !u.redirectAllowed(redirectUri) {
		return nil
	}
	reply := &authReply{redirectUri: redirectUri, state: params.Get("state"), issuer: u.frontIssuer(client), mode: "query"}
	if params.Get("response_mode") == "fragment" {
		reply.mode = "fragment"
	}
	return reply
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

// parseMaxAge validates max_age: a non-negative number of seconds; -1 when
// there is none.
func parseMaxAge(raw string) (int64, bool) {
	if raw == "" {
		return -1, true
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value < 0 {
		return 0, false
	}
	return value, true
}
