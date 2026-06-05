package app

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	appModel "github.com/rendau/ruto/internal/domain/app/model"
	endpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
	rootModel "github.com/rendau/ruto/internal/domain/root/model"
	sessionModel "github.com/rendau/ruto/internal/domain/session/model"
	varsModel "github.com/rendau/ruto/internal/domain/vars/model"
	"github.com/rendau/ruto/internal/errs"
	swaggerService "github.com/rendau/ruto/internal/service/swagger"
)

type testSessionService struct {
	session *sessionModel.Session
}

func (s *testSessionService) FromContext(_ context.Context) *sessionModel.Session {
	return s.session
}

func (s *testSessionService) CtxIsAuthorized(_ context.Context) bool {
	return s.session.IsAuthorized()
}

func (s *testSessionService) CtxIsAdmin(_ context.Context) bool {
	return s.session.IsAdmin()
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

type testEditAppService struct {
	list   func(ctx context.Context, pars *appModel.ListReq) ([]*appModel.App, int64, error)
	create func(ctx context.Context, obj *appModel.App) (string, error)
	update func(ctx context.Context, id string, obj *appModel.App) error
}

func (s *testEditAppService) List(ctx context.Context, pars *appModel.ListReq) ([]*appModel.App, int64, error) {
	return s.list(ctx, pars)
}

func (s *testEditAppService) Get(_ context.Context, _ string, _ bool) (*appModel.App, bool, error) {
	panic("unexpected call")
}

func (s *testEditAppService) Create(ctx context.Context, obj *appModel.App) (string, error) {
	return s.create(ctx, obj)
}

func (s *testEditAppService) Update(ctx context.Context, id string, obj *appModel.App) error {
	return s.update(ctx, id, obj)
}

func (s *testEditAppService) Delete(_ context.Context, _ string) error {
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

type testRootService struct {
	get func(ctx context.Context) (*rootModel.Root, error)
}

func (s *testRootService) Get(ctx context.Context) (*rootModel.Root, error) {
	return s.get(ctx)
}

func TestUsecase_Interpolate_NotAuthorized(t *testing.T) {

	uc := New(nil, nil, nil, &testSessionService{session: &sessionModel.Session{Id: 0}})
	_, err := uc.Interpolate(context.Background(), "app-id", varsModel.Vars{"k": "v"})
	require.ErrorIs(t, err, errs.NotAuthorized)
}

func TestUsecase_Inherited_NotAuthorized(t *testing.T) {

	uc := New(nil, nil, nil, &testSessionService{session: &sessionModel.Session{Id: 0}})
	_, err := uc.Inherited(context.Background(), "app-id", varsModel.Vars{"k": "v"})
	require.ErrorIs(t, err, errs.NotAuthorized)
}

func TestUsecase_Interpolate(t *testing.T) {

	uc := New(
		&testAppService{
			get: func(_ context.Context, id string, errNE bool) (*appModel.App, bool, error) {
				require.Equal(t, "app-id", id)
				require.True(t, errNE)
				return &appModel.App{
					Id: "app-id",
					Variables: varsModel.Vars{
						"app_var": "app-v",
					},
					Backend: appModel.Backend{
						Headers: varsModel.Vars{
							"X-Root": "{{root_var}}",
							"X-Req":  "{{req_var}}",
							"X-App":  "{{app_var}}",
						},
					},
				}, true, nil
			},
		},
		nil,
		nil,
		&testSessionService{session: &sessionModel.Session{Id: 1}},
		&testRootService{
			get: func(_ context.Context) (*rootModel.Root, error) {
				return &rootModel.Root{
					Variables: varsModel.Vars{
						"root_var": "root-v",
						"req_var":  "root-default",
					},
				}, nil
			},
		},
	)

	item, err := uc.Interpolate(context.Background(), " app-id ", varsModel.Vars{
		"req_var": "req-v",
	})
	require.NoError(t, err)
	require.Equal(t, "app-id", item.Id)
	require.Equal(t, varsModel.Vars{
		"req_var":  "req-v",
		"root_var": "root-v",
	}, item.Variables)
	require.Equal(t, varsModel.Vars{
		"X-Root": "root-v",
		"X-Req":  "req-v",
		"X-App":  "{{app_var}}",
	}, item.Backend.Headers)
}

func TestUsecase_Inherited(t *testing.T) {

	uc := New(
		&testAppService{
			get: func(_ context.Context, id string, errNE bool) (*appModel.App, bool, error) {
				require.Equal(t, "app-id", id)
				require.True(t, errNE)
				return &appModel.App{
					Id: "app-id",
					Variables: varsModel.Vars{
						"app_var": "app-v",
					},
					Backend: appModel.Backend{
						Headers: varsModel.Vars{
							"X-Root": "{{root_var}}",
							"X-Req":  "{{req_var}}",
							"X-App":  "{{app_var}}",
						},
					},
				}, true, nil
			},
		},
		nil,
		nil,
		&testSessionService{session: &sessionModel.Session{Id: 1}},
		&testRootService{
			get: func(_ context.Context) (*rootModel.Root, error) {
				return &rootModel.Root{
					Variables: varsModel.Vars{
						"root_var": "root-v",
						"req_var":  "root-default",
					},
				}, nil
			},
		},
	)

	item, err := uc.Inherited(context.Background(), " app-id ", varsModel.Vars{
		"req_var": "req-v",
	})
	require.NoError(t, err)
	require.Equal(t, "app-id", item.Id)
	require.Equal(t, varsModel.Vars{
		"req_var":  "req-v",
		"root_var": "root-v",
	}, item.Variables)
	require.Equal(t, varsModel.Vars{
		"X-Root": "{{root_var}}",
		"X-Req":  "{{req_var}}",
		"X-App":  "{{app_var}}",
	}, item.Backend.Headers)
}

func TestUsecase_GetSwaggerEndpointsDiff_NotAuthorized(t *testing.T) {

	uc := New(nil, nil, nil, &testSessionService{session: &sessionModel.Session{Id: 0}})
	_, err := uc.GetSwaggerEndpointsDiff(context.Background(), "app-id")
	require.ErrorIs(t, err, errs.NotAuthorized)
}

func TestUsecase_GetSwaggerEndpointsDiff(t *testing.T) {

	uc := New(
		&testAppService{
			get: func(_ context.Context, id string, _ bool) (*appModel.App, bool, error) {
				require.Equal(t, "app-id", id)
				return &appModel.App{
					Id: "app-id",
					Backend: appModel.Backend{
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
					{Http: endpointModel.Http{Method: "GET", Path: "users"}},
					{Http: endpointModel.Http{Method: "*", Path: "status"}},
					{Http: endpointModel.Http{Method: "DELETE", Path: "ghost"}},
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

func TestUsecase_GetSwaggerEndpointsDiff_PathVariableNamesIgnored(t *testing.T) {

	uc := New(
		&testAppService{
			get: func(_ context.Context, id string, _ bool) (*appModel.App, bool, error) {
				require.Equal(t, "app-id", id)
				return &appModel.App{
					Id: "app-id",
					Backend: appModel.Backend{
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
					{Http: endpointModel.Http{Method: "GET", Path: "/users/{id}"}},
					{Http: endpointModel.Http{Method: "POST", Path: "/users/:userId/orders/{order_id}"}},
				}, 2, nil
			},
		},
		&testSwaggerService{
			loadEndpoints: func(_ context.Context, swaggerURL string) ([]swaggerService.Endpoint, error) {
				require.Equal(t, "https://example.local/swagger.json", swaggerURL)
				return []swaggerService.Endpoint{
					{Method: "GET", Path: "/users/{userId}"},
					{Method: "POST", Path: "/users/{id}/orders/{id2}"},
				}, nil
			},
		},
		&testSessionService{session: &sessionModel.Session{Id: 1}},
	)

	rep, err := uc.GetSwaggerEndpointsDiff(context.Background(), "app-id")
	require.NoError(t, err)
	require.Empty(t, rep.Unregistered)
	require.Empty(t, rep.RegisteredInvalid)
}

func TestUsecase_Create_DuplicateAppName(t *testing.T) {

	createCalled := false
	uc := New(
		&testEditAppService{
			list: func(_ context.Context, pars *appModel.ListReq) ([]*appModel.App, int64, error) {
				require.EqualValues(t, 1, pars.PageSize)
				require.NotNil(t, pars.NameEqCI)
				require.Equal(t, "gateway api", *pars.NameEqCI)
				require.Nil(t, pars.ExcludeID)
				return []*appModel.App{
					{Id: "app-1", Name: "Gateway API"},
				}, 1, nil
			},
			create: func(_ context.Context, _ *appModel.App) (string, error) {
				createCalled = true
				return "new-id", nil
			},
			update: func(_ context.Context, _ string, _ *appModel.App) error {
				panic("unexpected call")
			},
		},
		nil,
		nil,
		&testSessionService{session: &sessionModel.Session{Id: 1}},
	)

	_, err := uc.Create(context.Background(), &appModel.App{
		Active:     true,
		PathPrefix: "/gateway",
		Name:       "gateway api",
		Backend: appModel.Backend{
			Url: "https://example.local",
		},
	})
	require.Error(t, err)
	require.False(t, createCalled)
	require.Contains(t, err.Error(), "app name must be unique")
}

func TestUsecase_Update_SameAppNameForSelfAllowed(t *testing.T) {

	updateCalled := false
	uc := New(
		&testEditAppService{
			list: func(_ context.Context, pars *appModel.ListReq) ([]*appModel.App, int64, error) {
				require.EqualValues(t, 1, pars.PageSize)
				require.NotNil(t, pars.NameEqCI)
				require.Equal(t, "gateway api", *pars.NameEqCI)
				require.NotNil(t, pars.ExcludeID)
				require.Equal(t, "app-1", *pars.ExcludeID)
				return nil, 0, nil
			},
			create: func(_ context.Context, _ *appModel.App) (string, error) {
				panic("unexpected call")
			},
			update: func(_ context.Context, id string, _ *appModel.App) error {
				updateCalled = true
				require.Equal(t, "app-1", id)
				return nil
			},
		},
		nil,
		nil,
		&testSessionService{session: &sessionModel.Session{Id: 1}},
	)

	err := uc.Update(context.Background(), "app-1", &appModel.App{
		Active:     true,
		PathPrefix: "/gateway",
		Name:       "gateway api",
		Backend: appModel.Backend{
			Url: "https://example.local",
		},
	})
	require.NoError(t, err)
	require.True(t, updateCalled)
}
