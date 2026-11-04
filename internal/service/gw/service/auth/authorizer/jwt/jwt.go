package jwt

import (
	"errors"
	"log/slog"
	"strings"

	jwtv5 "github.com/golang-jwt/jwt/v5"
	"github.com/samber/lo"

	"github.com/rendau/ruto/internal/constant"
	domAuthModel "github.com/rendau/ruto/internal/domain/auth/model"
	"github.com/rendau/ruto/internal/service/gw/service/auth/model"
	"github.com/rendau/ruto/internal/service/gw/service/jwk"
)

var jwtError = errors.New("jwt error")

type Jwt struct {
	jwkGetter       jwk.PublicKeyGetter
	provider        string
	cookieName      string // empty when the method pins no provider: no cookie to read
	requiredKid     string
	requiredRoleMap map[string]bool
}

func New(conf *domAuthModel.AuthMethodJWT, jwkGetter jwk.PublicKeyGetter) *Jwt {
	cookieName := ""
	if conf.Provider != "" {
		cookieName = constant.JwtCookieName(conf.Provider)
	}

	return &Jwt{
		jwkGetter:   jwkGetter,
		provider:    conf.Provider,
		cookieName:  cookieName,
		requiredKid: conf.Kid,
		requiredRoleMap: lo.SliceToMap(
			conf.Roles,
			func(role string) (string, bool) {
				return role, true
			},
		),
	}
}

func (a *Jwt) Authorize(req *model.AuthRequest) model.Result {
	jwtToken := req.ExtractToken()
	fromCookie := false
	if jwtToken == "" && a.cookieName != "" {
		jwtToken = req.ExtractCookie(a.cookieName)
		fromCookie = true
	}
	if jwtToken == "" {
		return model.NoCredentials
	}

	var kid string
	claims := jwtv5.MapClaims{}
	parsed, err := jwtv5.ParseWithClaims(jwtToken, claims,
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
		return model.InvalidCredentials
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

	if len(a.requiredRoleMap) > 0 {
		if !hasAnyRole(claims, a.checkRole) {
			return model.DeniedAuthenticated
		}
	}

	if fromCookie {
		req.MarkCookieToken(jwtToken)
	}

	return model.Allowed
}

func (a *Jwt) checkRole(role string) bool {
	return a.requiredRoleMap[role]
}
