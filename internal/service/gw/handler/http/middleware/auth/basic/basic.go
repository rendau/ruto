package basic

import (
	"crypto/subtle"
	"net/http"

	endpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
)

type Authorizer struct {
	conf *endpointModel.AuthMethodBasic
}

func New(conf *endpointModel.AuthMethodBasic) *Authorizer {
	return &Authorizer{conf: conf}
}

func (a Authorizer) Authorize(r *http.Request) bool {
	if len(a.conf.Users) == 0 {
		return false
	}

	username, password, ok := r.BasicAuth()
	if !ok {
		return false
	}

	for _, user := range a.conf.Users {
		if subtle.ConstantTimeCompare([]byte(username), []byte(user.Username)) != 1 {
			continue
		}
		if subtle.ConstantTimeCompare([]byte(password), []byte(user.Password)) == 1 {
			return true
		}
	}

	return false
}
