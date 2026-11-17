package broker

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/rendau/ruto/internal/service/gw/handler/http/authflow"
	gwAuthModel "github.com/rendau/ruto/internal/service/gw/service/auth/model"
	"github.com/rendau/ruto/internal/service/gw/service/oidc"
)

// userinfo is the userinfo endpoint: the provider's userinfo asked with the
// front token (from the front's cookie, got anew by an exchange when it
// expired), the answer passed on as it is. The token in Authorization only
// has to be a ruto token of the same person as the session; the session is in
// the cookies.
//
// Without a token in Authorization (a front opening a page without tokens in
// memory) the answer is built from the cookies alone and, at a live session,
// carries a "tokens" object with ruto's tokens for the front — so that the
// front's SDK goes on without a redirect to the authorization endpoint. Any
// other provider answers 401 to such a request, and the SDK logs in as usual.
func (u *unit) userinfo(w http.ResponseWriter, r *http.Request, client string, meta *oidc.Metadata) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		methodNotAllowed(w, "GET, POST")
		return
	}
	token := bearer(r)
	endpoint := meta.UserinfoEndpoint // on the internal address when one is set
	if endpoint == "" {
		http.NotFound(w, r)
		return
	}

	session, err := u.sessions.Session(w, r)
	switch {
	case err == nil:
	case errors.Is(err, authflow.ErrNoSession):
		unauthorized(w, token != "", "no login session")
		return
	default:
		slog.Warn("broker: session is unavailable", "provider", u.name, "client", client, "error", err)
		http.Error(w, "login provider is unavailable", http.StatusBadGateway)
		return
	}
	sub, _ := session.Claims["sub"].(string)

	if token != "" {
		claims, ok := u.rutoTokenClaims(token, client)
		if !ok || !gwAuthModel.IsRutoToken(token) {
			unauthorized(w, true, "the token is not a ruto token of this front")
			return
		}
		if got, _ := claims["sub"].(string); got != sub {
			unauthorized(w, true, "the token belongs to another user than the session")
			return
		}
	}

	frontToken, data, err := u.frontToken(w, r, client, session, sub)
	switch {
	case err == nil:
	case errors.Is(err, errSessionRejected):
		unauthorized(w, token != "", "the login session is over")
		return
	default:
		slog.Warn("broker: front token failed", "provider", u.name, "client", client, "error", err)
		http.Error(w, "login provider is unavailable", http.StatusBadGateway)
		return
	}

	rep, err := u.deps.Userinfo.Userinfo(r.Context(), endpoint, frontToken)
	if err != nil {
		slog.Warn("broker: provider userinfo failed", "provider", u.name, "client", client, "error", err)
		http.Error(w, "login provider is unavailable", http.StatusBadGateway)
		return
	}

	if token != "" || rep.Status != http.StatusOK {
		if rep.ContentType != "" {
			w.Header().Set("Content-Type", rep.ContentType)
		}
		w.WriteHeader(rep.Status)
		_, _ = w.Write(rep.Body)
		return
	}

	profile := map[string]json.RawMessage{}
	if err = json.Unmarshal(rep.Body, &profile); err != nil {
		slog.Warn("broker: provider userinfo is not a json object, no tokens added", "provider", u.name, "error", err)
		http.Error(w, "login provider answered userinfo in a form the gateway cannot extend", http.StatusBadGateway)
		return
	}
	tokens, err := u.issueTokens(client, &grant{Sub: sub, AuthTime: session.AuthTime}, session, data, false)
	if err != nil {
		slog.Error("broker: tokens not issued", "provider", u.name, "client", client, "error", err)
		http.Error(w, "the gateway cannot issue tokens", http.StatusInternalServerError)
		return
	}
	raw, _ := json.Marshal(tokens)
	profile["tokens"] = raw
	writeTokenJSON(w, http.StatusOK, profile)
}

// unauthorized answers 401 with the challenge of RFC 6750 §3: invalid_token
// when a token came, a bare one otherwise.
func unauthorized(w http.ResponseWriter, tokenSent bool, text string) {
	challenge := "Bearer"
	if tokenSent {
		challenge = `Bearer error="invalid_token"`
	}
	w.Header().Set("WWW-Authenticate", challenge)
	http.Error(w, text, http.StatusUnauthorized)
}
