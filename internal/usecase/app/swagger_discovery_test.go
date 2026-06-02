package app

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	sessionModel "github.com/rendau/ruto/internal/domain/session/model"
	"github.com/rendau/ruto/internal/errs"
	swaggerService "github.com/rendau/ruto/internal/service/swagger"
)

func TestUsecase_GetSwaggerURLByBackendURL_NotAuthorized(t *testing.T) {

	uc := New(nil, nil, nil, &testSessionService{session: &sessionModel.Session{Id: 0}})

	_, err := uc.GetSwaggerURLByBackendURL(context.Background(), "https://example.local")
	require.ErrorIs(t, err, errs.NotAuthorized)
}

func TestUsecase_GetSwaggerURLByBackendURL_InvalidBackendURL(t *testing.T) {

	uc := New(nil, nil, nil, &testSessionService{session: &sessionModel.Session{Id: 1}})

	_, err := uc.GetSwaggerURLByBackendURL(context.Background(), "example.local")
	require.ErrorIs(t, err, errs.InvalidRequest)
}

func TestUsecase_GetSwaggerURLByBackendURL_FindsByDocsSwaggerJSON(t *testing.T) {

	var attempted []string
	uc := New(
		nil,
		nil,
		&testSwaggerService{
			loadEndpoints: func(_ context.Context, swaggerURL string) ([]swaggerService.Endpoint, error) {
				attempted = append(attempted, swaggerURL)
				if swaggerURL == "https://example.local/service/docs/swagger.json" {
					return []swaggerService.Endpoint{
						{Method: "GET", Path: "/users"},
					}, nil
				}
				return nil, errors.New("not found")
			},
		},
		&testSessionService{session: &sessionModel.Session{Id: 1}},
	)

	result, err := uc.GetSwaggerURLByBackendURL(context.Background(), "https://example.local/service")
	require.NoError(t, err)
	require.Equal(t, "https://example.local/service/docs/swagger.json", result)
	require.NotEmpty(t, attempted)
}

func TestUsecase_GetSwaggerURLByBackendURL_SkipsEmptyEndpoints(t *testing.T) {

	uc := New(
		nil,
		nil,
		&testSwaggerService{
			loadEndpoints: func(_ context.Context, swaggerURL string) ([]swaggerService.Endpoint, error) {
				switch swaggerURL {
				case "https://example.local/service/docs":
					return []swaggerService.Endpoint{}, nil
				case "https://example.local/service/docs/swagger.json":
					return []swaggerService.Endpoint{
						{Method: "GET", Path: "/health"},
					}, nil
				default:
					return nil, errors.New("not found")
				}
			},
		},
		&testSessionService{session: &sessionModel.Session{Id: 1}},
	)

	result, err := uc.GetSwaggerURLByBackendURL(context.Background(), "https://example.local/service")
	require.NoError(t, err)
	require.Equal(t, "https://example.local/service/docs/swagger.json", result)
}

func TestUsecase_GetSwaggerURLByBackendURL_FindsByDocsApiSwaggerJSON(t *testing.T) {

	var attempted []string
	uc := New(
		nil,
		nil,
		&testSwaggerService{
			loadEndpoints: func(_ context.Context, swaggerURL string) ([]swaggerService.Endpoint, error) {
				attempted = append(attempted, swaggerURL)
				if swaggerURL == "https://example.local/service/docs/api.swagger.json" {
					return []swaggerService.Endpoint{
						{Method: "GET", Path: "/users"},
					}, nil
				}
				return nil, errors.New("not found")
			},
		},
		&testSessionService{session: &sessionModel.Session{Id: 1}},
	)

	result, err := uc.GetSwaggerURLByBackendURL(context.Background(), "https://example.local/service")
	require.NoError(t, err)
	require.Equal(t, "https://example.local/service/docs/api.swagger.json", result)
	require.NotEmpty(t, attempted)
}

func TestUsecase_GetSwaggerURLByBackendURL_FindsByDocsApiSwaggerYAML(t *testing.T) {

	var attempted []string
	uc := New(
		nil,
		nil,
		&testSwaggerService{
			loadEndpoints: func(_ context.Context, swaggerURL string) ([]swaggerService.Endpoint, error) {
				attempted = append(attempted, swaggerURL)
				if swaggerURL == "https://example.local/service/docs/api.swagger.yaml" {
					return []swaggerService.Endpoint{
						{Method: "GET", Path: "/users"},
					}, nil
				}
				return nil, errors.New("not found")
			},
		},
		&testSessionService{session: &sessionModel.Session{Id: 1}},
	)

	result, err := uc.GetSwaggerURLByBackendURL(context.Background(), "https://example.local/service")
	require.NoError(t, err)
	require.Equal(t, "https://example.local/service/docs/api.swagger.yaml", result)
	require.NotEmpty(t, attempted)
}

func TestUsecase_GetSwaggerURLByBackendURL_NotFound(t *testing.T) {

	uc := New(
		nil,
		nil,
		&testSwaggerService{
			loadEndpoints: func(_ context.Context, _ string) ([]swaggerService.Endpoint, error) {
				return nil, errors.New("not found")
			},
		},
		&testSessionService{session: &sessionModel.Session{Id: 1}},
	)

	result, err := uc.GetSwaggerURLByBackendURL(context.Background(), "https://example.local")
	require.NoError(t, err)
	require.Equal(t, "", result)
}
