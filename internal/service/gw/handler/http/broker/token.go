package broker

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/rendau/ruto/internal/constant"
	"github.com/rendau/ruto/internal/service/gw/handler/http/authflow"
	"github.com/rendau/ruto/internal/service/gw/service/apptoken"
)

// frontTokenMargin: a front token with less time left is got anew.
const frontTokenMargin = 10 * time.Second

var (
	errSessionRejected = errors.New("the provider does not take the shared token")
	errUnavailable     = errors.New("the provider is unavailable")
)

// token is the token endpoint. The code grant only for now: the refresh grant
// comes with the next stage.
func (u *unit) token(w http.ResponseWriter, r *http.Request, client string) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	if r.Method != http.MethodPost {
		methodNotAllowed(w, "POST")
		return
	}
	if err := r.ParseForm(); err != nil {
		tokenError(w, http.StatusBadRequest, "invalid_request", "the request is not readable")
		return
	}

	switch grant := r.PostForm.Get("grant_type"); grant {
	case "authorization_code":
		u.codeGrant(w, r, client)
	case "":
		tokenError(w, http.StatusBadRequest, "invalid_request", "grant_type is required")
	default:
		tokenError(w, http.StatusBadRequest, "unsupported_grant_type", "only authorization_code is supported")
	}
}

// codeGrant exchanges the broker's code (RFC 6749 §4.1.3, RFC 7636 §4.5) for
// ruto's tokens of the front, built on the front token: the shared token
// exchanged for the front's client, kept in the front's cookie.
func (u *unit) codeGrant(w http.ResponseWriter, r *http.Request, client string) {
	form := r.PostForm
	if form.Get("client_id") != client {
		tokenError(w, http.StatusUnauthorized, "invalid_client", "client_id must be the client of this address")
		return
	}
	code, ok := u.openCode(client, form.Get("code"))
	if !ok {
		tokenError(w, http.StatusBadRequest, "invalid_grant", "the code is not valid or has expired")
		return
	}
	if form.Get("redirect_uri") != code.RedirectUri {
		tokenError(w, http.StatusBadRequest, "invalid_grant", "redirect_uri differs from the one of the code")
		return
	}
	if !pkceMatches(form.Get("code_verifier"), code.Challenge) {
		tokenError(w, http.StatusBadRequest, "invalid_grant", "code_verifier does not match the code")
		return
	}
	if !u.deps.Used.MarkUsed(r.Context(), "code:"+code.Id, codeTTL) {
		tokenError(w, http.StatusBadRequest, "invalid_grant", "the code has already been used")
		return
	}

	session, err := u.sessions.Session(w, r)
	switch {
	case err == nil:
	case errors.Is(err, authflow.ErrNoSession):
		tokenError(w, http.StatusBadRequest, "invalid_grant", "no login session")
		return
	default:
		slog.Warn("broker: session is unavailable", "provider", u.name, "client", client, "error", err)
		tokenError(w, http.StatusServiceUnavailable, "temporarily_unavailable", "login provider is unavailable")
		return
	}
	if sub, _ := session.Claims["sub"].(string); sub != code.Sub {
		tokenError(w, http.StatusBadRequest, "invalid_grant", "the login session belongs to another user")
		return
	}

	data, err := u.frontToken(w, r, client, session, code.Sub)
	switch {
	case err == nil:
	case errors.Is(err, errSessionRejected):
		tokenError(w, http.StatusBadRequest, "invalid_grant", "the login session is over")
		return
	default:
		slog.Warn("broker: front token failed", "provider", u.name, "client", client, "error", err)
		tokenError(w, http.StatusServiceUnavailable, "temporarily_unavailable", "login provider is unavailable")
		return
	}

	rep, err := u.issueTokens(client, code, session, data)
	if err != nil {
		slog.Error("broker: tokens not issued", "provider", u.name, "client", client, "error", err)
		tokenError(w, http.StatusInternalServerError, "server_error", "the gateway cannot issue tokens")
		return
	}
	writeTokenJSON(w, http.StatusOK, rep)
}

// frontToken returns the claims ruto's tokens of the front are built on: the
// front token from its cookie when it is live and of the same person,
// otherwise the shared token exchanged for the front's client, put into the
// front's cookie and into the cookies of the apps whose audience is the
// client (one exchange for both). A provider that refuses the exchange on the
// merits (the person has nothing in this front) leaves the shared token's
// claims: the front logs in without its roles and tells the person so.
func (u *unit) frontToken(w http.ResponseWriter, r *http.Request, client string, session *authflow.Session, sub string) (jwt.MapClaims, error) {
	cookieName := constant.JwtFrontCookieName(u.name, client)
	if cookie, err := r.Cookie(cookieName); err == nil && cookie.Value != "" {
		if claims, ok := u.liveFrontToken(cookie.Value, client, sub); ok {
			return claims, nil
		}
	}

	token, err := u.deps.Exchanger.Exchange(r.Context(), u.name, session.AccessToken, client)
	switch {
	case err == nil:
	case errors.Is(err, apptoken.ErrRefused):
		slog.Info("broker: the provider gives no token for the front, the shared one serves",
			"provider", u.name, "client", client, "error", err)
		return session.Claims, nil
	case errors.Is(err, apptoken.ErrTokenRejected):
		return nil, fmt.Errorf("%w: %w", errSessionRejected, err)
	default:
		return nil, fmt.Errorf("%w: %w", errUnavailable, err)
	}

	claims, err := u.sessions.ParseProviderToken(token)
	if err != nil {
		return nil, fmt.Errorf("%w: exchanged token: %w", errUnavailable, err)
	}
	if !audienceHas(claims, client) {
		slog.Warn("broker: the exchanged token is not for the front, the shared one serves",
			"provider", u.name, "client", client)
		return session.Claims, nil
	}
	if got, _ := claims["sub"].(string); got != sub {
		return nil, fmt.Errorf("%w: exchanged token is for sub %q", errUnavailable, got)
	}
	exp, err := claims.GetExpirationTime()
	if err != nil || exp == nil {
		return nil, fmt.Errorf("%w: exchanged token has no exp", errUnavailable)
	}

	maxAge := int(time.Until(exp.Time).Seconds())
	if maxAge > 0 {
		u.sessions.SetCookie(w, cookieName, token, constant.BrokerPath(u.name, client), maxAge)
		for _, c := range u.audienceCookies[client] {
			u.sessions.SetCookie(w, c.Name, token, c.Path, maxAge)
		}
	}
	return claims, nil
}

// liveFrontToken checks a front token from the cookie: the provider's, for
// the front's client, of the same person, with frontTokenMargin to spare.
func (u *unit) liveFrontToken(token, client, sub string) (jwt.MapClaims, bool) {
	claims, err := u.sessions.ParseProviderToken(token)
	if err != nil || !audienceHas(claims, client) {
		return nil, false
	}
	if got, _ := claims["sub"].(string); got != sub {
		return nil, false
	}
	exp, err := claims.GetExpirationTime()
	if err != nil || exp == nil || time.Until(exp.Time) <= frontTokenMargin {
		return nil, false
	}
	return claims, true
}

func audienceHas(claims jwt.MapClaims, audience string) bool {
	aud, err := claims.GetAudience()
	return err == nil && slices.Contains(aud, audience)
}

func tokenError(w http.ResponseWriter, status int, code, description string) {
	writeTokenJSON(w, status, map[string]string{"error": code, "error_description": description})
}

func writeTokenJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
