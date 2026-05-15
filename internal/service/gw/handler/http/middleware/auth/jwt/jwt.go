package jwt

import (
	"fmt"
	"net/http"
	"strings"

	jwtv5 "github.com/golang-jwt/jwt/v5"
	"github.com/samber/lo"

	authModel "github.com/rendau/ruto/internal/domain/auth/model"
	"github.com/rendau/ruto/internal/service/gw/handler/http/request"
)

type Jwt struct {
	conf            *authModel.AuthMethodJWT
	requiredKidMap  map[string]bool
	requiredRoleMap map[string]bool
}

func New(conf *authModel.AuthMethodJWT) *Jwt {
	return &Jwt{
		conf: conf,
		requiredKidMap: lo.SliceToMap(
			conf.Kids,
			func(kid string) (string, bool) {
				return kid, true
			},
		),
		requiredRoleMap: lo.SliceToMap(
			conf.Roles,
			func(role string) (string, bool) {
				return role, true
			},
		),
	}
}

var (
	validJWTAlg = []string{
		jwtv5.SigningMethodRS256.Alg(),
		jwtv5.SigningMethodRS384.Alg(),
		jwtv5.SigningMethodRS512.Alg(),
	}
	validJWTAlgMap = lo.SliceToMap(
		validJWTAlg,
		func(method string) (string, bool) {
			return method, true
		},
	)
)

func (a Jwt) Authorize(r *http.Request) bool {
	ctxReq := request.Extract(r.Context())
	if ctxReq == nil {
		return false
	}

	token := extractToken(r)
	if token == "" {
		return false
	}

	claims := jwtv5.MapClaims{}
	parsed, err := jwtv5.ParseWithClaims(token, claims,
		func(tokenObj *jwtv5.Token) (any, error) {
			alg := strings.TrimSpace(tokenObj.Method.Alg())
			if !a.checkAlg(alg) {
				return nil, fmt.Errorf("invalid jwt algorithm: %s", alg)
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

			keyItem := ctxReq.JwkService.Get(kid)
			if keyItem == nil {
				return nil, fmt.Errorf("JWK not found for kid: %s", kid)
			}
			if keyItem.Alg != "" && keyItem.Alg != alg {
				return nil, fmt.Errorf("JWK alg does not match JWT alg: %s != %s", keyItem.Alg, alg)
			}

			if keyItem.RSAPublicKey == nil {
				return nil, fmt.Errorf("JWK does not contain public key")
			}

			return keyItem.RSAPublicKey, nil
		},
		jwtv5.WithValidMethods(validJWTAlg),
	)
	if err != nil || parsed == nil || !parsed.Valid {
		return false
	}

	if !hasAnyRole(claims, a.checkRole) {
		return false
	}

	return true
}

func (a Jwt) checkAlg(alg string) bool {
	return validJWTAlgMap[alg]
}

func (a Jwt) checkKid(kid string) bool {
	if len(a.requiredKidMap) == 0 {
		return true
	}
	return a.requiredKidMap[kid]
}

func (a Jwt) checkRole(role string) bool {
	if len(a.requiredRoleMap) == 0 {
		return true
	}
	return a.requiredRoleMap[role]
}
