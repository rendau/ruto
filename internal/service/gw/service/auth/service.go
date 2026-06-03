package auth

import (
	"fmt"

	"github.com/samber/lo"

	domAppModel "github.com/rendau/ruto/internal/domain/app/model"
	domAuthModel "github.com/rendau/ruto/internal/domain/auth/model"
	domEndpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
	domRootModel "github.com/rendau/ruto/internal/domain/root/model"
	variableModel "github.com/rendau/ruto/internal/domain/variable/model"
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
) (*Service, error) {
	mergedAuth := ep.Auth
	mergedAuth.Merge(&root.Auth, &app.Auth)
	if !mergedAuth.Enabled {
		return nil, nil
	}

	variables, err := root.EffectiveVariables(app, ep)
	if err != nil {
		return nil, fmt.Errorf("variables: %w", err)
	}
	scope, err := variableModel.Resolve(variables)
	if err != nil {
		return nil, fmt.Errorf("variables: %w", err)
	}
	mergedAuth, err = variableModel.InterpolateAuth(mergedAuth, scope)
	if err != nil {
		return nil, fmt.Errorf("auth: %w", err)
	}

	methods := lo.FilterMap(mergedAuth.Methods, func(v *domAuthModel.AuthMethod, _ int) (*method, bool) {
		return newMethod(v)
	})
	if len(methods) == 0 {
		return nil, nil
	}

	return &Service{
		root:    root,
		app:     app,
		ep:      ep,
		methods: methods,
	}, nil
}

func (s *Service) Check(req *model.AuthRequest) bool {
	for _, x := range s.methods {
		if x.check(req) {
			return true
		}
	}
	return false
}
