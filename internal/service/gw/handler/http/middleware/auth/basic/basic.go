package basic

import (
	"crypto/subtle"
	"net/http"

	authModel "github.com/rendau/ruto/internal/domain/auth/model"
)

type Authorizer struct {
	conf *authModel.AuthMethodBasic
}

func New(conf *authModel.AuthMethodBasic) *Authorizer {
	return &Authorizer{conf: conf}
}

func (a *Authorizer) Authorize(r *http.Request) bool {
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
