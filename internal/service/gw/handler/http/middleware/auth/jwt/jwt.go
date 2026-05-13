package jwt

import (
	"encoding/base64"
	"net/http"
	"strings"
	"time"

	"github.com/goccy/go-json"
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

type header struct {
	Alg string `json:"alg"`
	Kid string `json:"kid"`
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

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return false
	}

	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return false
	}

	var tokenHeader header
	if err = json.Unmarshal(headerBytes, &tokenHeader); err != nil {
		return false
	}
	if tokenHeader.Kid == "" {
		return false
	}
	if len(a.conf.Kids) > 0 && !lo.Contains(a.conf.Kids, tokenHeader.Kid) {
		return false
	}

	keyItem := ctxReq.JwkService.Get(tokenHeader.Kid)
	if keyItem == nil {
		return false
	}

	if !verifySignature(parts, tokenHeader.Alg, keyItem) {
		return false
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return false
	}

	claims := make(map[string]any)
	if err = json.Unmarshal(payloadBytes, &claims); err != nil {
		return false
	}

	now := time.Now().UTC()

	expValue, hasExp, ok := getNumericClaim(claims["exp"])
	if hasExp && (!ok || now.Unix() >= int64(expValue)) {
		return false
	}

	nbfValue, hasNbf, ok := getNumericClaim(claims["nbf"])
	if hasNbf && (!ok || now.Unix() < int64(nbfValue)) {
		return false
	}

	if len(a.conf.Roles) > 0 && !hasAnyRole(claims, a.conf.Roles) {
		return false
	}

	return true
}
