package basic

import (
	"crypto/subtle"

	authModel "github.com/rendau/ruto/internal/domain/auth/model"
	"github.com/rendau/ruto/internal/service/gw/service/auth/model"
)

type Authorizer struct {
	conf *authModel.AuthMethodBasic
}

func New(conf *authModel.AuthMethodBasic) *Authorizer {
	return &Authorizer{conf: conf}
}

func (a *Authorizer) Authorize(req *model.AuthRequest) bool {
	if len(a.conf.Users) == 0 {
		return false
	}

	username, password := req.ExtractBasic()

	if username == "" {
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
