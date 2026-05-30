package auth

import (
	"github.com/samber/lo"

	domAppModel "github.com/rendau/ruto/internal/domain/app/model"
	domAuthModel "github.com/rendau/ruto/internal/domain/auth/model"
	domEndpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
	domRootModel "github.com/rendau/ruto/internal/domain/root/model"
	"github.com/rendau/ruto/internal/service/gw/service/auth/model"
)

type Service struct {
	root    *domRootModel.Root
	app     *domAppModel.App
	ep      *domEndpointModel.Endpoint
	methods []*method
}

func New(
	root *domRootModel.Root,
	app *domAppModel.App,
	ep *domEndpointModel.Endpoint,
) *Service {
	mergedAuth := ep.Auth
	mergedAuth.Merge(&root.Auth, &app.Auth)
	if !mergedAuth.Enabled {
		return nil
	}

	methods := lo.FilterMap(mergedAuth.Methods, func(v *domAuthModel.AuthMethod, _ int) (*method, bool) {
		return newMethod(v)
	})
	if len(methods) == 0 {
		return nil
	}

	return &Service{
		root:    root,
		app:     app,
		ep:      ep,
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
