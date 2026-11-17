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
	"time"

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
	// Expires is when the cookie runs out (unix seconds): the session cannot
	// be renewed past it. Zero in cookies set before it was kept.
	Expires int64 `json:"exp,omitempty"`
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

// LoginOptions are the front's wishes for the login, passed to the provider
// as they are (OpenID Connect Core §3.1.2.1); empty ones are left out. The
// caller has validated them.
type LoginOptions struct {
	Prompt    string
	MaxAge    string
	LoginHint string
	UiLocales string
	AcrValues string
}

// Login starts a login at the provider: remembers state, PKCE verifier, nonce
// and ret, where the browser goes after the login, in a login cookie of its
// own and sends the browser to the provider. ret is a path on the gateway;
// the provider's answer comes to Callback. On an error nothing is written:
// the provider is unavailable.
func (u *Unit) Login(w http.ResponseWriter, r *http.Request, ret string, opts *LoginOptions) error {
	meta, err := oidc.Ins().Metadata(r.Context(), u.provider.Issuer, u.provider.InternalUrl)
	if err != nil {
		u.logWarn("provider discovery failed", err, r)
		return fmt.Errorf("%w: discovery: %w", ErrUnavailable, err)
	}
	authUrl, err := url.Parse(meta.AuthorizationEndpoint)
	if err != nil {
		u.logWarn("authorization_endpoint is not a url", err, r)
		return fmt.Errorf("%w: authorization_endpoint: %w", ErrUnavailable, err)
	}

	st := &loginState{
		state:    randomToken(16),
		verifier: randomToken(32),
		nonce:    randomToken(16),
		ret:      ret,
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
	if opts != nil {
		for name, value := range map[string]string{
			"prompt":     opts.Prompt,
			"max_age":    opts.MaxAge,
			"login_hint": opts.LoginHint,
			"ui_locales": opts.UiLocales,
			"acr_values": opts.AcrValues,
		} {
			if value != "" {
				query.Set(name, value)
			}
		}
	}
	authUrl.RawQuery = query.Encode()

	u.setCookie(w, u.loginCookieName(st.state), st.encode(), "/", loginCookieMaxAge)
	http.Redirect(w, r, authUrl.String(), http.StatusFound)
	return nil
}

// LoginError is a login that did not happen. Code is a standard error code
// where the standard has one (the provider's own code as it is,
// invalid_grant, temporarily_unavailable, …) or state_mismatch / invalid_token
// for the gateway's own checks; it is safe to put into an address.
type LoginError struct {
	Code   string
	Status int // for an answer on the gateway itself
	Text   string
}

func (e *LoginError) Error() string {
	return e.Code + ": " + e.Text
}

// Callback finishes a login at the provider's return address: finds the
// login cookie by the state, exchanges the code, verifies the tokens and sets
// the session cookies on w; the caller sends the browser on. It returns the
// ret of the login, also on a failure (a *LoginError) when the login cookie
// was there: "" means the gateway does not know where the person came from.
func (u *Unit) Callback(w http.ResponseWriter, r *http.Request) (string, error) {
	query := r.URL.Query()
	state := query.Get("state")

	var st *loginState
	if state != "" {
		name := u.loginCookieName(state)
		if cookie, err := r.Cookie(name); err == nil {
			st, _ = decodeLoginState(cookie.Value)
			u.clearCookie(w, name, "/")
		}
	}
	ret := ""
	if st != nil {
		ret = st.ret
	}

	if code := query.Get("error"); code != "" {
		u.logWarn("provider returned an error", fmt.Errorf("%s: %s", code, query.Get("error_description")), r)
		return ret, &LoginError{Code: errorCode(code), Status: http.StatusUnauthorized, Text: "login failed: " + code}
	}
	code := query.Get("code")
	if state == "" || code == "" {
		return ret, &LoginError{Code: "invalid_request", Status: http.StatusBadRequest, Text: "state and code are required"}
	}
	if st == nil {
		return "", &LoginError{Code: "invalid_request", Status: http.StatusBadRequest, Text: "no login in progress for this state: start the login again"}
	}
	if subtle.ConstantTimeCompare([]byte(st.state), []byte(state)) != 1 {
		return ret, &LoginError{Code: "state_mismatch", Status: http.StatusBadRequest, Text: "state does not match the login in progress"}
	}

	meta, err := oidc.Ins().Metadata(r.Context(), u.provider.Issuer, u.provider.InternalUrl)
	if err != nil {
		u.logWarn("provider discovery failed", err, r)
		return ret, &LoginError{Code: "temporarily_unavailable", Status: http.StatusBadGateway, Text: "login provider is unavailable"}
	}

	tokens, err := oidc.Ins().Exchange(r.Context(), meta.TokenEndpoint, u.client, code, st.verifier, u.redirectUri())
	if err != nil {
		u.logWarn("code exchange failed", err, r)
		if refusal, ok := errors.AsType[*oidc.Error](err); ok {
			return ret, &LoginError{Code: errorCode(refusal.Code), Status: http.StatusUnauthorized, Text: "login failed: " + refusal.Code}
		}
		return ret, &LoginError{Code: "temporarily_unavailable", Status: http.StatusBadGateway, Text: "login provider is unavailable"}
	}

	if err = u.verifyIdToken(tokens.IdToken, st.nonce); err != nil {
		u.logWarn("id token from the provider is not acceptable", err, r)
		return ret, &LoginError{Code: "invalid_token", Status: http.StatusBadGateway, Text: "login provider returned an id token this gateway cannot verify"}
	}
	if err = u.verifyAccessToken(tokens.AccessToken); err != nil {
		u.logWarn("access token from the provider is not acceptable", err, r)
		return ret, &LoginError{Code: "invalid_token", Status: http.StatusBadGateway, Text: "login provider returned a token this gateway cannot verify"}
	}

	u.setSession(w, tokens)
	return ret, nil
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

// renew refreshes the session with the refresh token of session and sets
// fresh cookies; the error is ErrNoSession or ErrUnavailable, wrapped.
func (u *Unit) renew(w http.ResponseWriter, r *http.Request, session sessionCookie) (*oidc.TokenResponse, error) {
	if session.RefreshToken == "" {
		return nil, fmt.Errorf("%w: no refresh cookie", ErrNoSession)
	}

	meta, err := oidc.Ins().Metadata(r.Context(), u.provider.Issuer, u.provider.InternalUrl)
	if err != nil {
		u.logWarn("provider discovery failed", err, r)
		return nil, fmt.Errorf("%w: discovery: %w", ErrUnavailable, err)
	}

	tokens, err := oidc.Ins().Refresh(r.Context(), meta.TokenEndpoint, u.client, session.RefreshToken)
	if err != nil {
		if refusal, ok := errors.AsType[*oidc.Error](err); ok && refusal.Code == "invalid_grant" {
			// The cookies are left as they are: a refresh of the same token
			// in another tab or pod may have just set fresh ones, and the
			// browser applies responses in the order they arrive.
			slog.Info("authflow: session can no longer be refreshed", "unit", u.name, "error", err)
			return nil, fmt.Errorf("%w: %w", ErrNoSession, err)
		}
		u.logWarn("refresh failed", err, r)
		return nil, fmt.Errorf("%w: %w", ErrUnavailable, err)
	}

	if err = u.verifyAccessToken(tokens.AccessToken); err != nil {
		u.logWarn("access token from the provider is not acceptable", err, r)
		return nil, fmt.Errorf("%w: access token: %w", ErrUnavailable, err)
	}

	if tokens.IdToken != "" {
		if err = u.verifyRenewedIdToken(tokens.IdToken, session.IdToken); err != nil {
			u.logWarn("renewed id token from the provider is not acceptable", err, r)
			return nil, fmt.Errorf("%w: id token: %w", ErrUnavailable, err)
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
	return &renewed, nil
}

// setSession sets the access and refresh cookies from the provider's answer.
// They live as long as the session can be renewed, not as long as the access
// token: a browser must not drop them before a refresh.
func (u *Unit) setSession(w http.ResponseWriter, tokens *oidc.TokenResponse) {
	maxAge := tokens.RefreshExpiresIn
	if maxAge <= 0 {
		maxAge = defaultSessionMaxAge
	}
	u.setCookie(w, u.cookieName, tokens.AccessToken, "/", maxAge)
	if tokens.RefreshToken != "" || tokens.IdToken != "" {
		session := sessionCookie{
			RefreshToken: tokens.RefreshToken,
			IdToken:      tokens.IdToken,
			Expires:      time.Now().Unix() + int64(maxAge),
		}
		name := u.refreshCookieName()
		if value, ok := session.encode(name); ok {
			u.setCookie(w, name, value, u.basePath, maxAge)
		} else {
			slog.Warn("authflow: no cookie key, the session is issued without a refresh cookie", "unit", u.name)
		}
	}
}

// Logout drops the session cookies with the app tokens got for it and sends
// the browser to redirect (already checked by the caller) through the
// provider's end_session_endpoint when the provider has one and the session
// has its id token. state, when not empty, comes back with the redirect: the
// provider passes it on, otherwise it is added here.
func (u *Unit) Logout(w http.ResponseWriter, r *http.Request, redirect, state string) {
	session := u.readSession(r)
	u.clearCookie(w, u.cookieName, "/")
	u.clearCookie(w, u.refreshCookieName(), u.basePath)
	for _, c := range u.appCookies {
		u.clearCookie(w, c.Name, c.Path)
	}

	if session.IdToken != "" {
		meta, err := oidc.Ins().Metadata(r.Context(), u.provider.Issuer, u.provider.InternalUrl)
		if err != nil {
			u.logWarn("provider discovery failed, provider session is left as is", err, r)
		} else if endUrl, parseErr := url.Parse(meta.EndSessionEndpoint); parseErr == nil && meta.EndSessionEndpoint != "" {
			query := endUrl.Query()
			query.Set("id_token_hint", session.IdToken)
			query.Set("client_id", u.client.Id)
			query.Set("post_logout_redirect_uri", redirect)
			if state != "" {
				query.Set("state", state)
			}
			endUrl.RawQuery = query.Encode()
			http.Redirect(w, r, endUrl.String(), http.StatusFound)
			return
		}
	}

	if state != "" {
		if target, err := url.Parse(redirect); err == nil {
			query := target.Query()
			query.Set("state", state)
			target.RawQuery = query.Encode()
			redirect = target.String()
		}
	}
	http.Redirect(w, r, redirect, http.StatusFound)
}

// readSession decodes the refresh cookie; a missing or malformed one is an
// empty session.
func (u *Unit) readSession(r *http.Request) sessionCookie {
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
func (u *Unit) verifyIdToken(token, nonce string) error {
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
func (u *Unit) verifyRenewedIdToken(token, previous string) error {
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
func (u *Unit) parseIdToken(token string) (jwtv5.MapClaims, error) {
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
func (u *Unit) verifyAccessToken(token string) error {
	_, err := u.parseAccessToken(token)
	return err
}

// parseAccessToken runs the checks of verifyAccessToken and returns the claims.
func (u *Unit) parseAccessToken(token string) (jwtv5.MapClaims, error) {
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
func (u *Unit) providerKey(tokenObj *jwtv5.Token) (any, error) {
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
