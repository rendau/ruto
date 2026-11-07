package authflow

import (
	"crypto/subtle"
	"encoding/base64"
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
// later renewals and the id token the provider wants back on logout. Stored as
// base64url(json); httpOnly keeps it from page scripts, encrypting it is left
// for later.
type sessionCookie struct {
	RefreshToken string `json:"rt,omitempty"`
	IdToken      string `json:"idt,omitempty"`
}

func (c *sessionCookie) encode() string {
	raw, _ := json.Marshal(c)
	return base64.RawURLEncoding.EncodeToString(raw)
}

func decodeSessionCookie(value string) sessionCookie {
	var session sessionCookie
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err == nil {
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

	meta, err := oidc.Ins().Metadata(r.Context(), u.provider.Issuer, u.provider.InternalUrl)
	if err != nil {
		u.logWarn("provider discovery failed", err, r)
		http.Error(w, "login provider is unavailable", http.StatusBadGateway)
		return
	}

	st := &loginState{
		state:    randomToken(16),
		verifier: randomToken(32),
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
	query.Set("code_challenge", pkceChallenge(st.verifier))
	query.Set("code_challenge_method", "S256")
	authUrl.RawQuery = query.Encode()

	http.Redirect(w, r, authUrl.String(), http.StatusFound)
}

// callback finishes the flow: checks the state against the login cookie,
// exchanges the code, verifies the access token and sets the session cookies.
func (u *unit) callback(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	loginCookieName := u.loginCookieName()

	if code := query.Get("error"); code != "" {
		u.clearCookie(w, loginCookieName, "/")
		u.logWarn("provider returned an error", fmt.Errorf("%s: %s", code, query.Get("error_description")), r)
		http.Error(w, "login failed: "+code, http.StatusUnauthorized)
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
	if !ok || subtle.ConstantTimeCompare([]byte(st.state), []byte(state)) != 1 {
		http.Error(w, "state does not match the login in progress", http.StatusBadRequest)
		return
	}
	// Checked again: the cookie came from the browser, not from this process.
	redirect, ok := u.returnUrl(st.redirect)
	if !ok {
		http.Error(w, "redirect is not allowed", http.StatusBadRequest)
		return
	}

	meta, err := oidc.Ins().Metadata(r.Context(), u.provider.Issuer, u.provider.InternalUrl)
	if err != nil {
		u.logWarn("provider discovery failed", err, r)
		http.Error(w, "login provider is unavailable", http.StatusBadGateway)
		return
	}

	tokens, err := oidc.Ins().Exchange(r.Context(), meta.TokenEndpoint, u.client, code, st.verifier, u.redirectUri())
	if err != nil {
		u.logWarn("code exchange failed", err, r)
		if refusal, ok := errors.AsType[*oidc.Error](err); ok {
			http.Error(w, "login failed: "+refusal.Code, http.StatusUnauthorized)
			return
		}
		http.Error(w, "login provider is unavailable", http.StatusBadGateway)
		return
	}

	if err = u.verifyAccessToken(tokens.AccessToken); err != nil {
		u.logWarn("access token from the provider is not acceptable", err, r)
		http.Error(w, "login provider returned a token this gateway cannot verify", http.StatusBadGateway)
		return
	}

	u.setSession(w, tokens)
	http.Redirect(w, r, redirect, http.StatusSeeOther)
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

	// A provider that does not rotate refresh tokens leaves them out of the
	// answer, and the id token is optional on a refresh: keep what we had.
	if tokens.RefreshToken == "" {
		tokens.RefreshToken = session.RefreshToken
	}
	if tokens.IdToken == "" {
		tokens.IdToken = session.IdToken
	}

	u.setSession(w, tokens)
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
		u.setCookie(w, u.refreshCookieName(), session.encode(), u.basePath, maxAge)
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

// readSession decodes the refresh cookie; a missing or malformed one is an
// empty session.
func (u *unit) readSession(r *http.Request) sessionCookie {
	cookie, err := r.Cookie(u.refreshCookieName())
	if err != nil {
		return sessionCookie{}
	}
	return decodeSessionCookie(cookie.Value)
}

// verifyAccessToken makes sure the provider handed us a token the jwt auth
// method will accept later: signed by a known key of this provider, issued by
// the configured issuer, with an expiry.
func (u *unit) verifyAccessToken(token string) error {
	_, err := jwtv5.Parse(token,
		func(tokenObj *jwtv5.Token) (any, error) {
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
		},
		jwtv5.WithIssuer(u.provider.Issuer),
		jwtv5.WithExpirationRequired(),
	)
	return err
}
