package app

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	appModel "github.com/rendau/ruto/internal/domain/app/model"
	sessionModel "github.com/rendau/ruto/internal/domain/session/model"
	swaggerService "github.com/rendau/ruto/internal/service/swagger"
)

type testBackfillAppService struct {
	list   func(ctx context.Context, pars *appModel.ListReq) ([]*appModel.App, int64, error)
	update func(ctx context.Context, id string, obj *appModel.App) error
}

func (s *testBackfillAppService) List(ctx context.Context, pars *appModel.ListReq) ([]*appModel.App, int64, error) {
	return s.list(ctx, pars)
}

func (s *testBackfillAppService) Get(_ context.Context, _ string, _ bool) (*appModel.App, bool, error) {
	panic("unexpected call")
}

func (s *testBackfillAppService) Create(_ context.Context, _ *appModel.App) (string, error) {
	panic("unexpected call")
}

func (s *testBackfillAppService) Update(ctx context.Context, id string, obj *appModel.App) error {
	return s.update(ctx, id, obj)
}

func (s *testBackfillAppService) Delete(_ context.Context, _ string) error {
	panic("unexpected call")
}

func TestUsecase_BackfillSwaggerURLs(t *testing.T) {

	var updates []*appModel.App
	svc := &testBackfillAppService{
		list: func(_ context.Context, pars *appModel.ListReq) ([]*appModel.App, int64, error) {
			if pars.NameEqCI != nil {
				require.NotNil(t, pars.ExcludeID)
				return nil, 0, nil
			}
			require.EqualValues(t, 0, pars.Page)
			return []*appModel.App{
				{
					Id:         "a1",
					PathPrefix: "/svc1",
					Name:       "Service One",
					Backend: appModel.Backend{
						Url: "https://example.local/service",
					},
				},
				{
					Id:         "a2",
					PathPrefix: "/svc2",
					Name:       "Service Two",
					Backend: appModel.Backend{
						Url: "",
					},
				},
				{
					Id:         "a3",
					PathPrefix: "/svc3",
					Name:       "Service Three",
					Backend: appModel.Backend{
						Url:        "https://example.local/already",
						SwaggerUrl: "https://example.local/already/swagger.json",
					},
				},
			}, 3, nil
		},
		update: func(_ context.Context, _ string, obj *appModel.App) error {
			updates = append(updates, new(*obj))
			return nil
		},
	}

	uc := New(
		svc,
		nil,
		&testSwaggerService{
			loadEndpoints: func(_ context.Context, swaggerURL string) ([]swaggerService.Endpoint, error) {
				if swaggerURL == "https://example.local/service" {
					return []swaggerService.Endpoint{
						{Method: "GET", Path: "/health"},
					}, nil
				}
				return nil, errors.New("not found")
			},
		},
		&testSessionService{session: &sessionModel.Session{Id: 1}},
	)

	err := uc.BackfillSwaggerURLs(context.Background())
	require.NoError(t, err)
	require.Len(t, updates, 1)
	require.Equal(t, "a1", updates[0].Id)
	require.Equal(t, "https://example.local/service", updates[0].Backend.SwaggerUrl)
}
