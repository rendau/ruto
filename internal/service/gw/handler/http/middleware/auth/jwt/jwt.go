package jwt

import (
	"fmt"
	"net/http"
	"strings"

	jwtv5 "github.com/golang-jwt/jwt/v5"
	"github.com/samber/lo"

	endpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
	"github.com/rendau/ruto/internal/service/gw/handler/http/request"
)

type Jwt struct {
	conf *endpointModel.AuthMethodJWT
}

func New(conf *endpointModel.AuthMethodJWT) *Jwt {
	return &Jwt{conf: conf}
}

var validJWTMethods = []string{
	jwtv5.SigningMethodRS256.Alg(),
	jwtv5.SigningMethodRS384.Alg(),
	jwtv5.SigningMethodRS512.Alg(),
}

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
			if !lo.Contains(validJWTMethods, alg) {
				return nil, fmt.Errorf("invalid jwt algorithm: %s", alg)
			}

			var kid string
			if rawKid, ok := tokenObj.Header["kid"].(string); ok {
				kid = strings.TrimSpace(rawKid)
			}
			if kid == "" {
				return nil, fmt.Errorf("missing kid in JWT header")
			}
			if len(a.conf.Kids) > 0 && !lo.Contains(a.conf.Kids, kid) {
				return nil, fmt.Errorf("kid not allowed: %s", kid)
			}

			keyItem := ctxReq.JwkService.Get(kid)
			if keyItem == nil {
				return nil, fmt.Errorf("JWK not found for kid: %s", kid)
			}
			if keyItem.Alg != "" && keyItem.Alg != alg {
				return nil, fmt.Errorf("JWK alg does not match JWT alg: %s != %s", keyItem.Alg, alg)
			}

			return jwkToRSAPublicKey(keyItem)
		},
		jwtv5.WithValidMethods(validJWTMethods),
	)
	if err != nil || parsed == nil || !parsed.Valid {
		return false
	}

	if len(a.conf.Roles) > 0 && !hasAnyRole(claims, a.conf.Roles) {
		return false
	}

	return true
}
