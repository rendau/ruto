package endpoint

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

func (s *testSessionService) CtxHasFullAppAccess(_ context.Context) bool {
	return s.session.IsAdmin() || (s.session.IsAuthorized() && s.session.AllApps)
}

func (s *testSessionService) CtxGetAppIds(_ context.Context) []string {
	return s.session.AppIds
}

type testEndpointService struct {
	get func(ctx context.Context, id string, errNE bool) (*endpointModel.Endpoint, bool, error)
}

func (s *testEndpointService) List(_ context.Context, _ *endpointModel.ListReq) ([]*endpointModel.Endpoint, int64, error) {
	panic("unexpected call")
}

func (s *testEndpointService) Get(ctx context.Context, id string, errNE bool) (*endpointModel.Endpoint, bool, error) {
	return s.get(ctx, id, errNE)
}

func (s *testEndpointService) Create(_ context.Context, _ *endpointModel.Endpoint) (string, error) {
	panic("unexpected call")
}

func (s *testEndpointService) Update(_ context.Context, _ string, _ *endpointModel.Endpoint) error {
	panic("unexpected call")
}

func (s *testEndpointService) Delete(_ context.Context, _ string) error {
	panic("unexpected call")
}

type testAppService struct {
	get func(ctx context.Context, id string, errNE bool) (*appModel.App, bool, error)
}

func (s *testAppService) Get(ctx context.Context, id string, errNE bool) (*appModel.App, bool, error) {
	return s.get(ctx, id, errNE)
}

type testRootService struct {
	get func(ctx context.Context) (*rootModel.Root, error)
}

func (s *testRootService) Get(ctx context.Context) (*rootModel.Root, error) {
	return s.get(ctx)
}

func TestUsecase_EndpointGet_RedactsForViewer(t *testing.T) {
	uc := New(
		&testEndpointService{
			get: func(_ context.Context, _ string, _ bool) (*endpointModel.Endpoint, bool, error) {
				return &endpointModel.Endpoint{
					Id:        "ep-1",
					AppId:     "app-1",
					Variables: varsModel.Vars{"secret": "secret-val"},
				}, true, nil
			},
		},
		// authorized non-owner
		&testSessionService{session: &sessionModel.Session{Id: 1}},
	)

	item, err := uc.Get(context.Background(), "ep-1")
	require.NoError(t, err)
	require.Equal(t, varsModel.RedactedPlaceholder, item.Variables["secret"])
}

func TestUsecase_EndpointGet_FullForOwner(t *testing.T) {
	uc := New(
		&testEndpointService{
			get: func(_ context.Context, _ string, _ bool) (*endpointModel.Endpoint, bool, error) {
				return &endpointModel.Endpoint{
					Id:        "ep-1",
					AppId:     "app-1",
					Variables: varsModel.Vars{"secret": "secret-val"},
				}, true, nil
			},
		},
		&testSessionService{session: &sessionModel.Session{Id: 1, AppIds: []string{"app-1"}}},
	)

	item, err := uc.Get(context.Background(), "ep-1")
	require.NoError(t, err)
	require.Equal(t, "secret-val", item.Variables["secret"])
}

func TestUsecase_EndpointDelete_ForeignApp_NoPermission(t *testing.T) {
	uc := New(
		&testEndpointService{
			get: func(_ context.Context, _ string, _ bool) (*endpointModel.Endpoint, bool, error) {
				return &endpointModel.Endpoint{Id: "ep-1", AppId: "app-1"}, true, nil
			},
		},
		&testSessionService{session: &sessionModel.Session{Id: 1}},
	)

	err := uc.Delete(context.Background(), "ep-1")
	require.ErrorIs(t, err, errs.NoPermission)
}

func TestUsecase_TestRequest_ForeignApp_NoPermission(t *testing.T) {
	uc := New(
		&testEndpointService{
			get: func(_ context.Context, _ string, _ bool) (*endpointModel.Endpoint, bool, error) {
				return &endpointModel.Endpoint{
					Id:    "ep-1",
					AppId: "app-1",
					Type:  endpointModel.TypeHTTP,
					Http:  endpointModel.Http{Method: "GET", Path: "users"},
				}, true, nil
			},
		},
		&testSessionService{session: &sessionModel.Session{Id: 1}},
		&testRootService{
			get: func(_ context.Context) (*rootModel.Root, error) {
				return &rootModel.Root{}, nil
			},
		},
		&testAppService{
			get: func(_ context.Context, _ string, _ bool) (*appModel.App, bool, error) {
				return &appModel.App{Id: "app-1", Backend: appModel.Backend{Url: "https://backend.local"}}, true, nil
			},
		},
	)

	_, err := uc.TestRequest(context.Background(), "ep-1", nil, nil, "")
	require.ErrorIs(t, err, errs.NoPermission)
}

func TestUsecase_EndpointInterpolate_NotAuthorized(t *testing.T) {
	uc := New(nil, &testSessionService{session: &sessionModel.Session{Id: 0}})

	_, err := uc.Interpolate(context.Background(), "ep-1", varsModel.Vars{"k": "v"})
	require.ErrorIs(t, err, errs.NotAuthorized)
}

func TestUsecase_EndpointInherited(t *testing.T) {
	uc := New(
		&testEndpointService{
			get: func(_ context.Context, id string, errNE bool) (*endpointModel.Endpoint, bool, error) {
				require.Equal(t, "ep-1", id)
				require.True(t, errNE)
				return &endpointModel.Endpoint{
					Id:    "ep-1",
					AppId: "app-1",
					Variables: varsModel.Vars{
						"ep": "ep-v",
					},
					Backend: endpointModel.Backend{
						Headers: varsModel.Vars{
							"X-Req":  "{{req}}",
							"X-App":  "{{app}}",
							"X-Root": "{{root}}",
							"X-Ep":   "{{ep}}",
						},
					},
				}, true, nil
			},
		},
		&testSessionService{session: &sessionModel.Session{Id: 1, AppIds: []string{"app-1"}}},
		&testRootService{
			get: func(_ context.Context) (*rootModel.Root, error) {
				return &rootModel.Root{
					Variables: varsModel.Vars{
						"root": "root-v",
						"req":  "root-default",
					},
				}, nil
			},
		},
		&testAppService{
			get: func(_ context.Context, id string, errNE bool) (*appModel.App, bool, error) {
				require.Equal(t, "app-1", id)
				require.True(t, errNE)
				return &appModel.App{
					Id: "app-1",
					Variables: varsModel.Vars{
						"app": "app-v",
						"req": "app-default",
					},
					Backend: appModel.Backend{
						Headers: varsModel.Vars{
							"X-From-App": "{{app}}",
						},
					},
				}, true, nil
			},
		},
	)

	item, err := uc.Inherited(context.Background(), " ep-1 ", varsModel.Vars{"req": "req-v"})
	require.NoError(t, err)
	require.Equal(t, varsModel.Vars{
		"req":  "req-v",
		"app":  "app-v",
		"root": "root-v",
	}, item.Variables)
	require.Equal(t, varsModel.Vars{
		"X-Req":      "{{req}}",
		"X-App":      "{{app}}",
		"X-Root":     "{{root}}",
		"X-Ep":       "{{ep}}",
		"X-From-App": "{{app}}",
	}, item.Backend.Headers)
}

func TestUsecase_EndpointInterpolate(t *testing.T) {
	uc := New(
		&testEndpointService{
			get: func(_ context.Context, _ string, _ bool) (*endpointModel.Endpoint, bool, error) {
				return &endpointModel.Endpoint{
					Id:    "ep-1",
					AppId: "app-1",
					Variables: varsModel.Vars{
						"ep": "ep-v",
					},
					Backend: endpointModel.Backend{
						Headers: varsModel.Vars{
							"X-Req":  "{{req}}",
							"X-App":  "{{app}}",
							"X-Root": "{{root}}",
							"X-Ep":   "{{ep}}",
						},
					},
				}, true, nil
			},
		},
		&testSessionService{session: &sessionModel.Session{Id: 1, AppIds: []string{"app-1"}}},
		&testRootService{
			get: func(_ context.Context) (*rootModel.Root, error) {
				return &rootModel.Root{
					Variables: varsModel.Vars{
						"root": "root-v",
						"req":  "root-default",
					},
				}, nil
			},
		},
		&testAppService{
			get: func(_ context.Context, _ string, _ bool) (*appModel.App, bool, error) {
				return &appModel.App{
					Id: "app-1",
					Variables: varsModel.Vars{
						"app": "app-v",
						"req": "app-default",
					},
					Backend: appModel.Backend{
						Headers: varsModel.Vars{
							"X-From-App": "{{app}}",
						},
					},
				}, true, nil
			},
		},
	)

	item, err := uc.Interpolate(context.Background(), "ep-1", varsModel.Vars{"req": "req-v"})
	require.NoError(t, err)
	require.Equal(t, varsModel.Vars{
		"req":  "req-v",
		"app":  "app-v",
		"root": "root-v",
	}, item.Variables)
	require.Equal(t, varsModel.Vars{
		"X-Req":      "req-v",
		"X-App":      "app-v",
		"X-Root":     "root-v",
		"X-Ep":       "{{ep}}",
		"X-From-App": "app-v",
	}, item.Backend.Headers)
}
