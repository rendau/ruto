package app

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	appModel "github.com/rendau/ruto/internal/domain/app/model"
	endpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
	sessionModel "github.com/rendau/ruto/internal/domain/session/model"
	"github.com/rendau/ruto/internal/errs"
	swaggerService "github.com/rendau/ruto/internal/service/swagger"
)

type testSessionService struct {
	session *sessionModel.Session
}

func (s *testSessionService) FromContext(_ context.Context) *sessionModel.Session {
	return s.session
}

type testAppService struct {
	get func(ctx context.Context, id string, errNE bool) (*appModel.App, bool, error)
}

func (s *testAppService) List(_ context.Context, _ *appModel.ListReq) ([]*appModel.App, int64, error) {
	panic("unexpected call")
}

func (s *testAppService) Get(ctx context.Context, id string, errNE bool) (*appModel.App, bool, error) {
	return s.get(ctx, id, errNE)
}

func (s *testAppService) Create(_ context.Context, _ *appModel.App) (string, error) {
	panic("unexpected call")
}

func (s *testAppService) Update(_ context.Context, _ string, _ *appModel.App) error {
	panic("unexpected call")
}

func (s *testAppService) Delete(_ context.Context, _ string) error {
	panic("unexpected call")
}

type testEndpointService struct {
	list func(ctx context.Context, pars *endpointModel.ListReq) ([]*endpointModel.Endpoint, int64, error)
}

func (s *testEndpointService) List(ctx context.Context, pars *endpointModel.ListReq) ([]*endpointModel.Endpoint, int64, error) {
	return s.list(ctx, pars)
}

type testSwaggerService struct {
	loadEndpoints func(ctx context.Context, swaggerURL string) ([]swaggerService.Endpoint, error)
}

func (s *testSwaggerService) LoadEndpoints(ctx context.Context, swaggerURL string) ([]swaggerService.Endpoint, error) {
	return s.loadEndpoints(ctx, swaggerURL)
}

func TestUsecase_GetSwaggerEndpointsDiff_NotAuthorized(t *testing.T) {
	t.Parallel()

	uc := New(nil, nil, nil, &testSessionService{session: &sessionModel.Session{Id: 0}})
	_, err := uc.GetSwaggerEndpointsDiff(context.Background(), "app-id")
	require.ErrorIs(t, err, errs.NotAuthorized)
}

func TestUsecase_GetSwaggerEndpointsDiff(t *testing.T) {
	t.Parallel()

	uc := New(
		&testAppService{
			get: func(_ context.Context, id string, _ bool) (*appModel.App, bool, error) {
				require.Equal(t, "app-id", id)
				return &appModel.App{
					Id: "app-id",
					Backend: appModel.AppBackend{
						SwaggerUrl: "https://example.local/swagger.json",
					},
				}, true, nil
			},
		},
		&testEndpointService{
			list: func(_ context.Context, pars *endpointModel.ListReq) ([]*endpointModel.Endpoint, int64, error) {
				require.NotNil(t, pars.AppId)
				require.Equal(t, "app-id", *pars.AppId)
				return []*endpointModel.Endpoint{
					{Method: "GET", Path: "users"},
					{Method: "*", Path: "status"},
					{Method: "DELETE", Path: "ghost"},
				}, 3, nil
			},
		},
		&testSwaggerService{
			loadEndpoints: func(_ context.Context, swaggerURL string) ([]swaggerService.Endpoint, error) {
				require.Equal(t, "https://example.local/swagger.json", swaggerURL)
				return []swaggerService.Endpoint{
					{Method: "GET", Path: "/users"},
					{Method: "POST", Path: "/users"},
					{Method: "GET", Path: "/status"},
				}, nil
			},
		},
		&testSessionService{session: &sessionModel.Session{Id: 1}},
	)

	rep, err := uc.GetSwaggerEndpointsDiff(context.Background(), "app-id")
	require.NoError(t, err)
	require.Equal(t, []SwaggerEndpoint{
		{Method: "GET", Path: "/status"},
		{Method: "POST", Path: "/users"},
	}, rep.Unregistered)
	require.Equal(t, []SwaggerEndpoint{
		{Method: "DELETE", Path: "/ghost"},
		{Method: "*", Path: "/status"},
	}, rep.RegisteredInvalid)
}
