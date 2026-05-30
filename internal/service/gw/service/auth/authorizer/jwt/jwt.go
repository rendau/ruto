package jwt

import (
	"errors"
	"strings"

	jwtv5 "github.com/golang-jwt/jwt/v5"
	"github.com/samber/lo"

	domAuthModel "github.com/rendau/ruto/internal/domain/auth/model"
	"github.com/rendau/ruto/internal/service/gw/service/auth/model"
	"github.com/rendau/ruto/internal/service/gw/service/jwk"
)

var jwtError = errors.New("jwt error")

type Jwt struct {
	jwkGetter       jwk.PublicKeyGetter
	requiredKid     string
	requiredRoleMap map[string]bool
}

func New(conf *domAuthModel.AuthMethodJWT, jwkGetter jwk.PublicKeyGetter) *Jwt {
	return &Jwt{
		jwkGetter:   jwkGetter,
		requiredKid: conf.Kid,
		requiredRoleMap: lo.SliceToMap(
			conf.Roles,
			func(role string) (string, bool) {
				return role, true
			},
		),
	}
}

func (a *Jwt) Authorize(req *model.AuthRequest) bool {
	jwtToken := req.ExtractToken()
	if jwtToken == "" {
		return false
	}

	jwkPubKey, jwkAlg := a.jwkGetter.GetPublicKey(a.requiredKid)
	if jwkPubKey == nil {
		return false
	}

	claims := jwtv5.MapClaims{}
	parsed, err := jwtv5.ParseWithClaims(jwtToken, claims,
		func(tokenObj *jwtv5.Token) (any, error) {
			// check kid
			kid, ok := tokenObj.Header["kid"].(string)
			if !ok {
				return nil, jwtError
			}
			kid = strings.TrimSpace(kid)
			if kid == "" || kid != a.requiredKid {
				return nil, jwtError
			}

			return jwkPubKey, nil
		},
		jwtv5.WithValidMethods([]string{jwkAlg}),
	)
	if err != nil || parsed == nil || !parsed.Valid {
		return false
	}

	if len(a.requiredRoleMap) > 0 {
		if !hasAnyRole(claims, a.checkRole) {
			return false
		}
	}

	return true
}

func (a *Jwt) checkRole(role string) bool {
	return a.requiredRoleMap[role]
}
