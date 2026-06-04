package auth

import (
	"github.com/samber/lo"

	domAuthModel "github.com/rendau/ruto/internal/domain/auth/model"
	domEndpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
	"github.com/rendau/ruto/internal/service/gw/service/auth/model"
)

type Service struct {
	methods []*method
}

func New(
	ep *domEndpointModel.Endpoint,
) *Service {
	if !ep.Auth.Enabled {
		return nil
	}

	methods := lo.FilterMap(ep.Auth.Methods, func(v *domAuthModel.AuthMethod, _ int) (*method, bool) {
		return newMethod(v)
	})
	if len(methods) == 0 {
		return nil
	}

	return &Service{
		methods: methods,
	}
}

func (s *Service) Check(req *model.AuthRequest) bool {
	for _, x := range s.methods {
		if x.check(req) {
			return true
		}
	}
	return false
}
