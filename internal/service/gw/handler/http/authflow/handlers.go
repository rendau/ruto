package authflow

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	jwtv5 "github.com/golang-jwt/jwt/v5"

	"github.com/rendau/ruto/internal/service/gw/service/jwk"
	"github.com/rendau/ruto/internal/service/gw/service/oidc"
)

// sessionCookie is the content of the refresh cookie: the refresh token for
// later renewals and the id token the provider wants back on logout. Stored
// as JSON sealed with the cookie key from core (see sealed.go): httpOnly keeps
// it from page scripts, the seal from everything that can read the cookie.
type sessionCookie struct {
	RefreshToken string `json:"rt,omitempty"`
	IdToken      string `json:"idt,omitempty"`
}

// encode seals the session for the cookie called name; false without a key.
func (c *sessionCookie) encode(name string) (string, bool) {
	raw, _ := json.Marshal(c)
	return seal(name, raw)
}

// decodeSessionCookie opens the value of the cookie called name; anything not
// sealed for it with the current key is an empty session.
func decodeSessionCookie(name, value string) sessionCookie {
	var session sessionCookie
	if raw, ok := open(name, value); ok {
		_ = json.Unmarshal(raw, &session)
	}
	return session
}

// login starts the flow: remembers state, PKCE verifier and the return address
// in the login cookie and sends the browser to the provider.
func (u *unit) login(w http.ResponseWriter, r *http.Request) {
	redirect, ok := u.returnUrl(r.URL.Query().Get("redirect"))
	if !ok {
		http.Error(w, "redirect is not allowed: not this gateway and not a CORS origin", http.StatusBadRequest)
		return
	}
	prompt, ok := parsePrompt(r.URL.Query().Get("prompt"))
	if !ok {
		http.Error(w, "prompt is not valid: none, login, consent or select_account, space separated, none alone", http.StatusBadRequest)
		return
	}

	meta, err := oidc.Ins().Metadata(r.Context(), u.provider.Issuer, u.provider.InternalUrl)
	if err != nil {
		u.logWarn("provider discovery failed", err, r)
		http.Error(w, "login provider is unavailable", http.StatusBadGateway)
		return
	}

	st := &loginState{
		state:    randomToken(16),
		verifier: randomToken(32),
		nonce:    randomToken(16),
		redirect: redirect,
	}
	u.setCookie(w, u.loginCookieName(), st.encode(), "/", loginCookieMaxAge)

	authUrl, err := url.Parse(meta.AuthorizationEndpoint)
	if err != nil {
		u.logWarn("authorization_endpoint is not a url", err, r)
		http.Error(w, "login provider is misconfigured", http.StatusBadGateway)
		return
	}
	query := authUrl.Query()
	query.Set("response_type", "code")
	query.Set("client_id", u.client.Id)
	query.Set("redirect_uri", u.redirectUri())
	query.Set("scope", strings.Join(u.client.Scopes, " "))
	query.Set("state", st.state)
	query.Set("nonce", st.nonce)
	query.Set("code_challenge", pkceChallenge(st.verifier))
	query.Set("code_challenge_method", "S256")
	if prompt != "" {
		query.Set("prompt", prompt)
	}
	authUrl.RawQuery = query.Encode()

	http.Redirect(w, r, authUrl.String(), http.StatusFound)
}

// promptValues are the values OpenID Connect Core §3.1.2.1 defines for the
// prompt parameter. A front passes prompt through login as it is: "login"
// asks the provider for the credentials again even with a live SSO session
// (another person wants to log in after an access denial), "select_account"
// for an account picker, "consent" for the consent screen, "none" for no
// interaction at all.
var promptValues = map[string]bool{"none": true, "login": true, "consent": true, "select_account": true}

// parsePrompt validates a prompt query value: standard values separated by
// spaces, "none" only alone. Empty means no prompt.
func parsePrompt(raw string) (string, bool) {
	values := strings.Fields(raw)
	for _, value := range values {
		if !promptValues[value] || (value == "none" && len(values) > 1) {
			return "", false
		}
	}
	return strings.Join(values, " "), true
}

// callback finishes the flow: checks the state against the login cookie,
// exchanges the code, verifies the access token and sets the session cookies.
// A failure goes back to the front with an error code (see loginFailed).
func (u *unit) callback(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	loginCookieName := u.loginCookieName()

	if code := query.Get("error"); code != "" {
		redirect := ""
		if cookie, err := r.Cookie(loginCookieName); err == nil {
			if st, ok := decodeLoginState(cookie.Value); ok {
				redirect, _ = u.returnUrl(st.redirect)
			}
		}
		u.clearCookie(w, loginCookieName, "/")
		u.logWarn("provider returned an error", fmt.Errorf("%s: %s", code, query.Get("error_description")), r)
		u.loginFailed(w, r, redirect, errorCode(code), http.StatusUnauthorized, "login failed: "+code)
		return
	}

	state, code := query.Get("state"), query.Get("code")
	if state == "" || code == "" {
		http.Error(w, "state and code are required", http.StatusBadRequest)
		return
	}

	cookie, err := r.Cookie(loginCookieName)
	if err != nil {
		http.Error(w, "no login in progress: start again from /login", http.StatusBadRequest)
		return
	}
	st, ok := decodeLoginState(cookie.Value)
	u.clearCookie(w, loginCookieName, "/")
	if !ok {
		http.Error(w, "state does not match the login in progress", http.StatusBadRequest)
		return
	}
	// Checked again: the cookie came from the browser, not from this process.
	redirect, ok := u.returnUrl(st.redirect)
	if !ok {
		http.Error(w, "redirect is not allowed", http.StatusBadRequest)
		return
	}
	if subtle.ConstantTimeCompare([]byte(st.state), []byte(state)) != 1 {
		u.loginFailed(w, r, redirect, "state_mismatch", http.StatusBadRequest, "state does not match the login in progress")
		return
	}

	meta, err := oidc.Ins().Metadata(r.Context(), u.provider.Issuer, u.provider.InternalUrl)
	if err != nil {
		u.logWarn("provider discovery failed", err, r)
		u.loginFailed(w, r, redirect, "temporarily_unavailable", http.StatusBadGateway, "login provider is unavailable")
		return
	}

	tokens, err := oidc.Ins().Exchange(r.Context(), meta.TokenEndpoint, u.client, code, st.verifier, u.redirectUri())
	if err != nil {
		u.logWarn("code exchange failed", err, r)
		if refusal, ok := errors.AsType[*oidc.Error](err); ok {
			u.loginFailed(w, r, redirect, errorCode(refusal.Code), http.StatusUnauthorized, "login failed: "+refusal.Code)
			return
		}
		u.loginFailed(w, r, redirect, "temporarily_unavailable", http.StatusBadGateway, "login provider is unavailable")
		return
	}

	if err = u.verifyIdToken(tokens.IdToken, st.nonce); err != nil {
		u.logWarn("id token from the provider is not acceptable", err, r)
		u.loginFailed(w, r, redirect, "invalid_token", http.StatusBadGateway, "login provider returned an id token this gateway cannot verify")
		return
	}
	if err = u.verifyAccessToken(tokens.AccessToken); err != nil {
		u.logWarn("access token from the provider is not acceptable", err, r)
		u.loginFailed(w, r, redirect, "invalid_token", http.StatusBadGateway, "login provider returned a token this gateway cannot verify")
		return
	}

	u.setSession(w, tokens)
	http.Redirect(w, r, redirect, http.StatusSeeOther)
}

// loginFailed ends a login that did not happen. When the return address of
// the login is known (the login cookie was there and valid), the browser goes
// back to it with `auth_error=<code>` in the query, so the front can tell the
// person what happened and offer to try again instead of leaving them on a
// bare gateway page; the standard covers only how the provider reports an
// error to the gateway, this leg is ours, with the standard's codes where
// they exist (access_denied, invalid_grant, temporarily_unavailable, …) and
// state_mismatch / invalid_token for the gateway's own checks. Without a
// return address the text answer stays: there is nowhere to go.
func (u *unit) loginFailed(w http.ResponseWriter, r *http.Request, redirect, code string, status int, text string) {
	if redirect == "" {
		http.Error(w, text, status)
		return
	}
	target, err := url.Parse(redirect)
	if err != nil {
		http.Error(w, text, status)
		return
	}
	query := target.Query()
	query.Set("auth_error", code)
	target.RawQuery = query.Encode()
	http.Redirect(w, r, target.String(), http.StatusSeeOther)
}

// errorCode keeps a provider's error code as it is when it looks like one
// (a token such as access_denied or login_required) and hides anything else
// behind server_error: the value ends up in a front's address bar.
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

// refresh renews the session with the refresh token from the refresh cookie
// and sets fresh cookies. A front calls it after a 401 and repeats the
// request; a 401 from here means the session is over and it is time for login.
func (u *unit) refresh(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")

	session := u.readSession(r)
	if session.RefreshToken == "" {
		http.Error(w, "no session to refresh: log in", http.StatusUnauthorized)
		return
	}

	meta, err := oidc.Ins().Metadata(r.Context(), u.provider.Issuer, u.provider.InternalUrl)
	if err != nil {
		u.logWarn("provider discovery failed", err, r)
		http.Error(w, "login provider is unavailable", http.StatusBadGateway)
		return
	}

	tokens, err := oidc.Ins().Refresh(r.Context(), meta.TokenEndpoint, u.client, session.RefreshToken)
	if err != nil {
		if refusal, ok := errors.AsType[*oidc.Error](err); ok && refusal.Code == "invalid_grant" {
			// The cookies are left as they are: a refresh of the same token
			// in another tab or pod may have just set fresh ones, and the
			// browser applies responses in the order they arrive.
			slog.Info("authflow: session can no longer be refreshed", "unit", u.name, "error", err)
			http.Error(w, "session is over: log in", http.StatusUnauthorized)
			return
		}
		u.logWarn("refresh failed", err, r)
		http.Error(w, "login provider is unavailable", http.StatusBadGateway)
		return
	}

	if err = u.verifyAccessToken(tokens.AccessToken); err != nil {
		u.logWarn("access token from the provider is not acceptable", err, r)
		http.Error(w, "login provider returned a token this gateway cannot verify", http.StatusBadGateway)
		return
	}

	if tokens.IdToken != "" {
		if err = u.verifyRenewedIdToken(tokens.IdToken, session.IdToken); err != nil {
			u.logWarn("renewed id token from the provider is not acceptable", err, r)
			http.Error(w, "login provider returned an id token this gateway cannot verify", http.StatusBadGateway)
			return
		}
	}

	// A provider that does not rotate refresh tokens leaves them out of the
	// answer, and the id token is optional on a refresh: keep what we had. The
	// answer may be shared with other refreshes of the token, so fill a copy.
	renewed := *tokens
	if renewed.RefreshToken == "" {
		renewed.RefreshToken = session.RefreshToken
	}
	if renewed.IdToken == "" {
		renewed.IdToken = session.IdToken
	}

	u.setSession(w, &renewed)
	w.WriteHeader(http.StatusNoContent)
}

// setSession sets the access and refresh cookies from the provider's answer.
// They live as long as the session can be renewed, not as long as the access
// token: a browser must not drop them before a refresh.
func (u *unit) setSession(w http.ResponseWriter, tokens *oidc.TokenResponse) {
	maxAge := tokens.RefreshExpiresIn
	if maxAge <= 0 {
		maxAge = defaultSessionMaxAge
	}
	u.setCookie(w, u.cookieName, tokens.AccessToken, u.cookiePath, maxAge)
	if tokens.RefreshToken != "" || tokens.IdToken != "" {
		session := sessionCookie{RefreshToken: tokens.RefreshToken, IdToken: tokens.IdToken}
		name := u.refreshCookieName()
		if value, ok := session.encode(name); ok {
			u.setCookie(w, name, value, u.basePath, maxAge)
		} else {
			slog.Warn("authflow: no cookie key, the session is issued without a refresh cookie", "unit", u.name)
		}
	}
}

// logout drops the session cookies and, when the provider supports it, ends
// the provider session too (OpenID Connect RP-Initiated Logout).
func (u *unit) logout(w http.ResponseWriter, r *http.Request) {
	redirect, ok := u.returnUrl(r.URL.Query().Get("redirect"))
	if !ok {
		http.Error(w, "redirect is not allowed: not this gateway and not a CORS origin", http.StatusBadRequest)
		return
	}

	session := u.readSession(r)
	u.clearCookie(w, u.cookieName, u.cookiePath)
	u.clearCookie(w, u.refreshCookieName(), u.basePath)

	if session.IdToken != "" {
		meta, err := oidc.Ins().Metadata(r.Context(), u.provider.Issuer, u.provider.InternalUrl)
		if err != nil {
			u.logWarn("provider discovery failed, provider session is left as is", err, r)
		} else if endUrl, parseErr := url.Parse(meta.EndSessionEndpoint); parseErr == nil && meta.EndSessionEndpoint != "" {
			query := endUrl.Query()
			query.Set("id_token_hint", session.IdToken)
			query.Set("client_id", u.client.Id)
			query.Set("post_logout_redirect_uri", redirect)
			endUrl.RawQuery = query.Encode()
			http.Redirect(w, r, endUrl.String(), http.StatusFound)
			return
		}
	}

	http.Redirect(w, r, redirect, http.StatusFound)
}

// me tells a front who the access cookie of this unit belongs to: the claims
// of the access token, as they are. The gateway does not pick or rename them,
// providers put the username and the roles in different places and a front
// may need a claim of its own; what matters is that the token itself stays in
// the httpOnly cookie, since claims without a signature cannot be presented
// anywhere. Only the cookie is read, not the Authorization header: a client
// that holds a bearer token has the claims already. A missing, foreign or
// expired cookie is 401, which a front treats like any other 401: refresh,
// then repeat. The route lives under the cookie's path (mePath), or the
// browser would not send the cookie along.
func (u *unit) me(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")

	cookie, err := r.Cookie(u.cookieName)
	if err != nil || cookie.Value == "" {
		http.Error(w, "no session: log in", http.StatusUnauthorized)
		return
	}

	claims, err := u.parseAccessToken(cookie.Value)
	if err != nil {
		http.Error(w, "session is not valid: refresh or log in", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(claims)
}

// readSession decodes the refresh cookie; a missing or malformed one is an
// empty session.
func (u *unit) readSession(r *http.Request) sessionCookie {
	name := u.refreshCookieName()
	cookie, err := r.Cookie(name)
	if err != nil {
		return sessionCookie{}
	}
	return decodeSessionCookie(name, cookie.Value)
}

// verifyIdToken validates the id token as OpenID Connect Core §3.1.3.7 asks a
// client to: signed by a key of this provider, issued by the configured
// issuer, meant for our client (aud, and azp when present), not expired, and
// carrying the nonce this login started with. The scope always includes
// openid, so the provider must have sent one.
func (u *unit) verifyIdToken(token, nonce string) error {
	if token == "" {
		return errors.New("no id_token in the token response")
	}

	claims, err := u.parseIdToken(token)
	if err != nil {
		return err
	}

	got, _ := claims["nonce"].(string)
	if subtle.ConstantTimeCompare([]byte(got), []byte(nonce)) != 1 {
		return errors.New("nonce does not match the login in progress")
	}

	return nil
}

// verifyRenewedIdToken validates an id token that came with a refresh
// (OpenID Connect Core §12.2): the same checks as at login but for the
// nonce, and the same subject as the id token of the session. previous was
// verified at login; it is read as is, since a cookie its owner broke only
// hurts its owner.
func (u *unit) verifyRenewedIdToken(token, previous string) error {
	claims, err := u.parseIdToken(token)
	if err != nil {
		return err
	}

	if previous == "" {
		return nil
	}
	prev, _, err := jwtv5.NewParser().ParseUnverified(previous, jwtv5.MapClaims{})
	if err != nil {
		return nil
	}
	prevSub, _ := prev.Claims.(jwtv5.MapClaims)["sub"].(string)
	sub, _ := claims["sub"].(string)
	if sub != prevSub {
		return fmt.Errorf("sub %q differs from the session's %q", sub, prevSub)
	}

	return nil
}

// parseIdToken checks the signature by a key of this provider, iss, aud and
// azp against our client, and exp; the claims are returned for the checks
// that depend on the flow.
func (u *unit) parseIdToken(token string) (jwtv5.MapClaims, error) {
	claims := jwtv5.MapClaims{}
	if _, err := jwtv5.ParseWithClaims(token, claims, u.providerKey,
		jwtv5.WithIssuer(u.provider.Issuer),
		jwtv5.WithAudience(u.client.Id),
		jwtv5.WithExpirationRequired(),
	); err != nil {
		return nil, err
	}

	if azp, ok := claims["azp"].(string); ok && azp != "" && azp != u.client.Id {
		return nil, fmt.Errorf("azp %q is not our client", azp)
	}

	return claims, nil
}

// verifyAccessToken makes sure the provider handed us a token the jwt auth
// method will accept later: signed by a known key of this provider, issued by
// the configured issuer, with an expiry.
func (u *unit) verifyAccessToken(token string) error {
	_, err := u.parseAccessToken(token)
	return err
}

// parseAccessToken runs the checks of verifyAccessToken and returns the claims.
func (u *unit) parseAccessToken(token string) (jwtv5.MapClaims, error) {
	claims := jwtv5.MapClaims{}
	if _, err := jwtv5.ParseWithClaims(token, claims, u.providerKey,
		jwtv5.WithIssuer(u.provider.Issuer),
		jwtv5.WithExpirationRequired(),
	); err != nil {
		return nil, err
	}
	return claims, nil
}

// providerKey resolves the token's kid to a key of this provider and checks
// the token's alg against the key's.
func (u *unit) providerKey(tokenObj *jwtv5.Token) (any, error) {
	kid, _ := tokenObj.Header["kid"].(string)
	if kid == "" {
		return nil, errors.New("token has no kid")
	}
	key, alg := jwk.Ins().GetPublicKey(u.provider.Name, kid)
	if key == nil {
		return nil, fmt.Errorf("unknown key %q", kid)
	}
	if tokenObj.Method.Alg() != alg {
		return nil, fmt.Errorf("token alg %q does not match the key's %q", tokenObj.Method.Alg(), alg)
	}
	return key, nil
}
