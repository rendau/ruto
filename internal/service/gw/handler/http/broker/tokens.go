package broker

import (
	"fmt"
	"maps"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/rendau/ruto/internal/service/gw/handler/http/authflow"
)

// defaultSessionTTL stands for the end of a session whose cookie does not say
// when it runs out.
const defaultSessionTTL = 24 * time.Hour

// serviceClaims describe a token rather than the person: they are not copied
// from the provider's token into ruto's.
var serviceClaims = []string{
	"iss", "sub", "aud", "azp", "exp", "iat", "nbf", "jti", "auth_time", "nonce", "typ",
	"sid", "session_state", "scope", "at_hash", "c_hash", "s_hash", "allowed-origins",
	"cnf", "client_id", "act", "may_act",
}

// tokenResponse is the token endpoint's answer (RFC 6749 §5.1, OpenID Connect
// Core §3.1.3.3).
type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	IdToken      string `json:"id_token"`
	Scope        string `json:"scope,omitempty"`
}

// issueTokens builds ruto's tokens of the front. They live as long as the
// shared token: the front refreshes them when it would have to be renewed.
// The gateway does not take them as credentials (see auth model IsRutoToken);
// the person's data and roles in them, from the front token, are for the
// front's SDK, which reads them from the ID and the access token alike.
func (u *unit) issueTokens(client string, code *loginCode, session *authflow.Session, data jwt.MapClaims) (*tokenResponse, error) {
	now := time.Now()
	person := maps.Clone(data)
	for _, name := range serviceClaims {
		delete(person, name)
	}
	common := func(claims jwt.MapClaims, exp time.Time) jwt.MapClaims {
		claims["iss"] = u.frontIssuer(client)
		claims["sub"] = code.Sub
		claims["aud"] = client
		claims["azp"] = client
		claims["iat"] = now.Unix()
		claims["exp"] = exp.Unix()
		return claims
	}

	id := common(maps.Clone(person), session.Expires)
	if code.Nonce != "" {
		id["nonce"] = code.Nonce
	}
	if code.AuthTime > 0 {
		id["auth_time"] = code.AuthTime
	}

	access := common(maps.Clone(person), session.Expires)
	access["typ"] = "Bearer"
	access["jti"] = randomToken(16)
	if code.Scope != "" {
		access["scope"] = code.Scope
	}

	sessionEnd := session.SessionExpires
	if sessionEnd.IsZero() {
		sessionEnd = now.Add(defaultSessionTTL)
	}
	refresh := common(jwt.MapClaims{}, sessionEnd)
	refresh["typ"] = "Refresh"
	refresh["jti"] = randomToken(16)

	rep := &tokenResponse{TokenType: "Bearer", ExpiresIn: int(session.Expires.Sub(now).Seconds()), Scope: code.Scope}
	var err error
	if rep.IdToken, err = u.deps.Keys.Sign(id); err != nil {
		return nil, fmt.Errorf("id token: %w", err)
	}
	if rep.AccessToken, err = u.deps.Keys.Sign(access); err != nil {
		return nil, fmt.Errorf("access token: %w", err)
	}
	if rep.RefreshToken, err = u.deps.Keys.Sign(refresh); err != nil {
		return nil, fmt.Errorf("refresh token: %w", err)
	}
	return rep, nil
}
