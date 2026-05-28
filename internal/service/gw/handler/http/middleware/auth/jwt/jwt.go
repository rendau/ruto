package jwt

import (
	"fmt"
	"net/http"
	"strings"

	jwtv5 "github.com/golang-jwt/jwt/v5"
	"github.com/samber/lo"

	"github.com/rendau/ruto/internal/constant"
	authModel "github.com/rendau/ruto/internal/domain/auth/model"
)

type Jwt struct {
	jwkGetter       JwkGetterI
	conf            *authModel.AuthMethodJWT
	requiredKid     string
	requiredRoleMap map[string]bool
}

func New(jwkGetter JwkGetterI, conf *authModel.AuthMethodJWT) *Jwt {
	return &Jwt{
		jwkGetter:   jwkGetter,
		conf:        conf,
		requiredKid: strings.TrimSpace(conf.Kid),
		requiredRoleMap: lo.SliceToMap(
			conf.Roles,
			func(role string) (string, bool) {
				return role, true
			},
		),
	}
}

func (a *Jwt) Authorize(r *http.Request) bool {
	token := extractToken(r)
	if token == "" {
		return false
	}

	claims := jwtv5.MapClaims{}
	parsed, err := jwtv5.ParseWithClaims(token, claims,
		func(tokenObj *jwtv5.Token) (any, error) {
			jwtAlg := strings.TrimSpace(tokenObj.Method.Alg())
			if !a.checkAlg(jwtAlg) {
				return nil, fmt.Errorf("invalid jwt algorithm: %s", jwtAlg)
			}

			var kid string
			if rawKid, ok := tokenObj.Header["kid"].(string); ok {
				kid = strings.TrimSpace(rawKid)
			}
			if kid == "" {
				return nil, fmt.Errorf("missing kid in JWT header")
			}
			if !a.checkKid(kid) {
				return nil, fmt.Errorf("kid not allowed: %s", kid)
			}

			jwk, jwkAlg := a.jwkGetter.GetPublicKey(kid)
			if jwk == nil {
				return nil, fmt.Errorf("JWK not found for kid: %s", kid)
			}
			if jwkAlg != jwtAlg {
				return nil, fmt.Errorf("JWK alg does not match JWT alg: %s != %s", jwkAlg, jwtAlg)
			}

			return jwk, nil
		},
		jwtv5.WithValidMethods(constant.SupportedJWTAlgorithms),
	)
	if err != nil || parsed == nil || !parsed.Valid {
		return false
	}

	if len(a.requiredRoleMap) == 0 {
		return true
	}

	if !hasAnyRole(claims, a.checkRole) {
		return false
	}

	return true
}

func (a *Jwt) checkAlg(alg string) bool {
	return constant.IsSupportedJWTAlgorithm(alg)
}

func (a *Jwt) checkKid(kid string) bool {
	if a.requiredKid == "" {
		return false
	}
	return a.requiredKid == kid
}

func (a *Jwt) checkRole(role string) bool {
	if len(a.requiredRoleMap) == 0 {
		return true
	}
	return a.requiredRoleMap[role]
}
