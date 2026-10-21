package auth

import (
	configGateway "github.com/rendau/ruto/internal/config/gateway"
	authModel "github.com/rendau/ruto/internal/domain/auth/model"
	"github.com/rendau/ruto/internal/service/gw/service/auth/authorizer"
	"github.com/rendau/ruto/internal/service/gw/service/auth/authorizer/api_key"
	"github.com/rendau/ruto/internal/service/gw/service/auth/authorizer/basic"
	"github.com/rendau/ruto/internal/service/gw/service/auth/authorizer/ip_validation"
	"github.com/rendau/ruto/internal/service/gw/service/auth/authorizer/jwt"
	"github.com/rendau/ruto/internal/service/gw/service/auth/model"
	"github.com/rendau/ruto/internal/service/gw/service/jwk"
)

type method struct {
	authorizers []authorizer.Authorizer
}

func newMethod(src *authModel.AuthMethod) (*method, bool) {
	authorizers := make([]authorizer.Authorizer, 0, 4)

	if src.Basic != nil {
		authorizers = append(authorizers, basic.New(src.Basic))
	}
	if src.APIKey != nil {
		authorizers = append(authorizers, api_key.New(src.APIKey))
	}
	if src.JWT != nil {
		authorizers = append(authorizers, jwt.New(src.JWT, jwk.Ins()))
	}
	if src.IPValidation != nil {
		authorizers = append(authorizers, ip_validation.New(src.IPValidation, configGateway.Conf.TrustedProxyAddresses))
	}

	if len(authorizers) == 0 {
		return nil, false
	}

	return &method{
		authorizers: authorizers,
	}, true
}

func (m *method) check(req *model.AuthRequest) model.Result {
	for i, x := range m.authorizers {
		res := x.Authorize(req)
		if res == model.Allowed {
			continue
		}
		// ip validation goes after the credential authorizers, so its denial
		// after any of them passed refuses an authenticated client.
		if res == model.Denied && i > 0 {
			return model.DeniedAuthenticated
		}
		return res
	}
	return model.Allowed
}
