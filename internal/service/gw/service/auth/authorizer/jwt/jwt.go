package jwt

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	jwtv5 "github.com/golang-jwt/jwt/v5"

	"github.com/rendau/ruto/internal/constant"
	domAuthModel "github.com/rendau/ruto/internal/domain/auth/model"
	"github.com/rendau/ruto/internal/service/gw/service/apptoken"
	"github.com/rendau/ruto/internal/service/gw/service/auth/model"
	"github.com/rendau/ruto/internal/service/gw/service/jwk"
)

var jwtError = errors.New("jwt error")

// invalidTokenChallenge answers a request without a live shared token where
// the app token is exchanged for it (RFC 6750 §3): the front refreshes the
// session or logs in.
const invalidTokenChallenge = `Bearer error="invalid_token"`

// appTokenMargin: an app token that expires sooner is exchanged anew, so the
// backend does not get a token that dies on its way.
const appTokenMargin = 10 * time.Second

// Exchanger trades the shared token of a provider for an app token, see
// apptoken.Service.
type Exchanger interface {
	Exchange(ctx context.Context, provider, subjectToken, audience string) (string, error)
	CookieSecure(provider string) bool
}

type Jwt struct {
	jwkGetter   jwk.PublicKeyGetter
	exchanger   Exchanger
	provider    string
	cookieName  string // empty when the method pins no provider: no cookie to read
	exchange    *domAuthModel.JwtExchange
	requiredKid string
	// requiredRoleMap holds the bare roles, requiredClientRoles the
	// "client:role" ones by client; any one of them lets the token in.
	// A "client:*" entry stores anyClientRole: any role of that client is enough.
	requiredRoleMap     map[string]bool
	requiredClientRoles map[string]map[string]bool
}

const anyClientRole = "*"

func New(conf *domAuthModel.AuthMethodJWT, jwkGetter jwk.PublicKeyGetter, exchanger Exchanger) *Jwt {
	// The cookie name is set when the config is inherited; a bare method
	// falls back to the provider cookie.
	cookieName := conf.CookieName
	if cookieName == "" && conf.Provider != "" {
		cookieName = constant.JwtCookieName(conf.Provider)
	}

	a := &Jwt{
		jwkGetter:           jwkGetter,
		exchanger:           exchanger,
		provider:            conf.Provider,
		cookieName:          cookieName,
		requiredKid:         conf.Kid,
		requiredRoleMap:     make(map[string]bool, len(conf.Roles)),
		requiredClientRoles: make(map[string]map[string]bool),
	}
	if conf.Exchange != nil && conf.Provider != "" && exchanger != nil {
		a.exchange = conf.Exchange
	}
	for _, role := range conf.Roles {
		client, clientRole, ok := strings.Cut(role, ":")
		if !ok || client == "" || clientRole == "" {
			a.requiredRoleMap[role] = true
			continue
		}
		if a.requiredClientRoles[client] == nil {
			a.requiredClientRoles[client] = make(map[string]bool)
		}
		a.requiredClientRoles[client][clientRole] = true
	}

	return a
}

// Authorize lets in a token from Authorization (or auth_token) as it is. Without
// one the token comes from the cookies: the shared token of the provider, or,
// for an app with an audience of its own, the app token (see authorizeAppToken).
func (a *Jwt) Authorize(req *model.AuthRequest) model.Result {
	if token := req.ExtractToken(); token != "" {
		claims, err := a.parse(token)
		if err != nil {
			return model.InvalidCredentials
		}
		return a.checkRoles(claims)
	}
	if a.cookieName == "" {
		return model.NoCredentials
	}
	if a.exchange != nil {
		return a.authorizeAppToken(req)
	}

	token := req.ExtractCookie(a.cookieName)
	if token == "" {
		return model.NoCredentials
	}
	claims, err := a.parse(token)
	if err != nil {
		return model.InvalidCredentials
	}
	result := a.checkRoles(claims)
	if result == model.Allowed {
		req.MarkCookieToken(token)
	}
	return result
}

// authorizeAppToken lets the request in by the app token: a live one from the
// app's cookie, or a new one exchanged for the shared token and put into the
// cookies of every app with the same audience. The roles are the app token's;
// the backend gets the app token and not the shared one.
func (a *Jwt) authorizeAppToken(req *model.AuthRequest) model.Result {
	if token := req.ExtractCookie(a.exchange.Cookie.Name); token != "" {
		if claims, err := a.parseAppToken(token); err == nil && !expiresSoon(claims) {
			return a.letInByAppToken(req, token, claims)
		}
	}

	shared := req.ExtractCookie(a.cookieName)
	if shared == "" {
		req.SetChallenge(invalidTokenChallenge)
		return model.NoCredentials
	}
	if _, err := a.parse(shared); err != nil {
		req.SetChallenge(invalidTokenChallenge)
		return model.InvalidCredentials
	}

	token, err := a.exchanger.Exchange(req.Context(), a.provider, shared, a.exchange.Audience)
	if err != nil {
		switch {
		case errors.Is(err, apptoken.ErrTokenRejected):
			slog.Info("jwt: the provider rejected the shared token in an exchange",
				"provider", a.provider, "audience", a.exchange.Audience, "error", err)
			req.SetChallenge(invalidTokenChallenge)
			return model.InvalidCredentials
		case errors.Is(err, apptoken.ErrRefused):
			slog.Info("jwt: the provider refused the app token",
				"provider", a.provider, "audience", a.exchange.Audience, "error", err)
			return model.DeniedAuthenticated
		default:
			slog.Warn("jwt: app token exchange failed",
				"provider", a.provider, "audience", a.exchange.Audience, "error", err)
			return model.Unavailable
		}
	}

	claims, err := a.parseAppToken(token)
	if err != nil {
		if errors.Is(err, errNotForAudience) {
			// The provider issued a token, but not for the audience: the
			// user has nothing in this app.
			slog.Info("jwt: the exchanged token is not for the audience",
				"provider", a.provider, "audience", a.exchange.Audience)
			return model.DeniedAuthenticated
		}
		slog.Warn("jwt: the exchanged token cannot be verified",
			"provider", a.provider, "audience", a.exchange.Audience, "error", err)
		return model.Unavailable
	}
	a.keepAppToken(req, token, claims)

	return a.letInByAppToken(req, token, claims)
}

func (a *Jwt) letInByAppToken(req *model.AuthRequest, token string, claims jwtv5.MapClaims) model.Result {
	result := a.checkRoles(claims)
	if result == model.Allowed {
		req.MarkCookieToken(token)
		req.StripCookie(a.cookieName)
	}
	return result
}

// keepAppToken puts the token into the app's cookie and the cookies of the
// other apps with the same audience, for as long as it lives.
func (a *Jwt) keepAppToken(req *model.AuthRequest, token string, claims jwtv5.MapClaims) {
	exp, err := claims.GetExpirationTime()
	if err != nil || exp == nil {
		return
	}
	maxAge := int(time.Until(exp.Time) / time.Second)
	if maxAge <= 0 {
		return
	}
	secure := a.exchanger.CookieSecure(a.provider)
	for _, c := range slices.Concat([]domAuthModel.JwtCookie{a.exchange.Cookie}, a.exchange.Copies) {
		req.SetResponseCookie(&http.Cookie{
			Name:     c.Name,
			Value:    token,
			Path:     c.Path,
			MaxAge:   maxAge,
			HttpOnly: true,
			Secure:   secure,
			SameSite: http.SameSiteLaxMode,
		})
	}
}

func expiresSoon(claims jwtv5.MapClaims) bool {
	exp, err := claims.GetExpirationTime()
	return err != nil || exp == nil || time.Until(exp.Time) < appTokenMargin
}

var errNotForAudience = errors.New("token is not for the audience")

// parseAppToken checks an app token like any other and requires the audience
// among its aud and an expiry.
func (a *Jwt) parseAppToken(token string) (jwtv5.MapClaims, error) {
	claims, err := a.parse(token)
	if err != nil {
		return nil, err
	}
	aud, err := claims.GetAudience()
	if err != nil || !slices.Contains(aud, a.exchange.Audience) {
		return nil, errNotForAudience
	}
	if exp, err := claims.GetExpirationTime(); err != nil || exp == nil {
		return nil, jwtError
	}
	return claims, nil
}

// parse checks the signature by a key of the provider (the key pinned by kid
// when the method pins one) and the expiry when the token has one.
func (a *Jwt) parse(token string) (jwtv5.MapClaims, error) {
	var kid string
	claims := jwtv5.MapClaims{}
	parsed, err := jwtv5.ParseWithClaims(token, claims,
		func(tokenObj *jwtv5.Token) (any, error) {
			kid, _ = tokenObj.Header["kid"].(string)
			kid = strings.TrimSpace(kid)
			if kid == "" || (a.requiredKid != "" && kid != a.requiredKid) {
				return nil, jwtError
			}

			// The key is looked up by the token's kid, so the signing method
			// is checked against that key here rather than via WithValidMethods.
			jwkPubKey, jwkAlg := a.jwkGetter.GetPublicKey(a.provider, kid)
			if jwkPubKey == nil || tokenObj.Method.Alg() != jwkAlg {
				return nil, jwtError
			}

			return jwkPubKey, nil
		},
	)
	if err != nil || parsed == nil || !parsed.Valid {
		return nil, jwtError
	}

	if userId, firstSeen := markIndefinite(kid, claims); firstSeen {
		slog.Info(
			"indefinite jwt detected (no exp claim)",
			"user_id", userId,
			"provider", a.provider,
			"kid", kid,
			"payload", map[string]any(claims), // весь payload токена целиком
		)
	}

	return claims, nil
}

func (a *Jwt) checkRoles(claims jwtv5.MapClaims) model.Result {
	if (len(a.requiredRoleMap) > 0 || len(a.requiredClientRoles) > 0) && !a.hasRequiredRole(claims) {
		return model.DeniedAuthenticated
	}
	return model.Allowed
}

func (a *Jwt) hasRequiredRole(claims map[string]any) bool {
	if len(a.requiredRoleMap) > 0 && hasAnyRole(claims, a.checkRole) {
		return true
	}
	for client, roles := range a.requiredClientRoles {
		if hasClientRole(claims, client, func(role string) bool { return roles[anyClientRole] || roles[role] }) {
			return true
		}
	}
	return false
}

func (a *Jwt) checkRole(role string) bool {
	return a.requiredRoleMap[role]
}
