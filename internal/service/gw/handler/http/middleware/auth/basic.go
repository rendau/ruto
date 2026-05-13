package auth

import (
	"crypto/subtle"
	"net/http"

	endpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
)

type basicAuthorizer struct {
	conf *endpointModel.AuthMethodBasic
}

func newBasicAuthorizer(conf *endpointModel.AuthMethodBasic) authorizerI {
	return basicAuthorizer{conf: conf}
}

func (a basicAuthorizer) Authorize(r *http.Request) bool {
	return authorizeBasic(r, a.conf)
}

func authorizeBasic(r *http.Request, conf *endpointModel.AuthMethodBasic) bool {
	if len(conf.Users) == 0 {
		return false
	}

	username, password, ok := r.BasicAuth()
	if !ok {
		return false
	}

	for _, user := range conf.Users {
		if subtle.ConstantTimeCompare([]byte(username), []byte(user.Username)) != 1 {
			continue
		}
		if subtle.ConstantTimeCompare([]byte(password), []byte(user.Password)) == 1 {
			return true
		}
	}

	return false
}
