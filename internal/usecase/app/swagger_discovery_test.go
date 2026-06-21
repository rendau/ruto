package app

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	sessionModel "github.com/rendau/ruto/internal/domain/session/model"
	"github.com/rendau/ruto/internal/errs"
	swaggerService "github.com/rendau/ruto/internal/service/swagger"
)

// attemptRecorder потокобезопасно собирает URL-кандидаты: discoverSwaggerURL
// опрашивает их параллельно из нескольких воркеров.
type attemptRecorder struct {
	mu    sync.Mutex
	items []string
}

func (r *attemptRecorder) add(item string) {
	r.mu.Lock()
	r.items = append(r.items, item)
	r.mu.Unlock()
}

func (r *attemptRecorder) snapshot() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.items...)
}

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

	attempted := &attemptRecorder{}
	uc := New(
		nil,
		nil,
		&testSwaggerService{
			loadEndpoints: func(_ context.Context, swaggerURL string) ([]swaggerService.Endpoint, error) {
				attempted.add(swaggerURL)
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
	require.NotEmpty(t, attempted.snapshot())
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

	attempted := &attemptRecorder{}
	uc := New(
		nil,
		nil,
		&testSwaggerService{
			loadEndpoints: func(_ context.Context, swaggerURL string) ([]swaggerService.Endpoint, error) {
				attempted.add(swaggerURL)
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
	require.NotEmpty(t, attempted.snapshot())
}

func TestUsecase_GetSwaggerURLByBackendURL_FindsByDocsApiSwaggerYAML(t *testing.T) {

	attempted := &attemptRecorder{}
	uc := New(
		nil,
		nil,
		&testSwaggerService{
			loadEndpoints: func(_ context.Context, swaggerURL string) ([]swaggerService.Endpoint, error) {
				attempted.add(swaggerURL)
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
	require.NotEmpty(t, attempted.snapshot())
}

func TestUsecase_GetSwaggerURLByBackendURL_FindsOnSystemPort(t *testing.T) {

	uc := New(
		nil,
		nil,
		&testSwaggerService{
			loadEndpoints: func(_ context.Context, swaggerURL string) ([]swaggerService.Endpoint, error) {
				if swaggerURL == "https://example.local:3003/doc" {
					return []swaggerService.Endpoint{
						{Method: "GET", Path: "/users"},
					}, nil
				}
				return nil, errors.New("not found")
			},
		},
		&testSessionService{session: &sessionModel.Session{Id: 1}},
	)

	result, err := uc.GetSwaggerURLByBackendURL(context.Background(), "https://example.local:8080/api/v1")
	require.NoError(t, err)
	require.Equal(t, "https://example.local:3003/doc", result)
}

func TestUsecase_GetSwaggerURLByBackendURL_FindsOnSystemPortHTTPFallback(t *testing.T) {

	uc := New(
		nil,
		nil,
		&testSwaggerService{
			loadEndpoints: func(_ context.Context, swaggerURL string) ([]swaggerService.Endpoint, error) {
				if swaggerURL == "http://example.local:3003/swagger.json" {
					return []swaggerService.Endpoint{
						{Method: "GET", Path: "/health"},
					}, nil
				}
				return nil, errors.New("not found")
			},
		},
		&testSessionService{session: &sessionModel.Session{Id: 1}},
	)

	result, err := uc.GetSwaggerURLByBackendURL(context.Background(), "https://example.local:8080/api/v1")
	require.NoError(t, err)
	require.Equal(t, "http://example.local:3003/swagger.json", result)
}

func TestBuildSwaggerCandidates_IncludesSystemPort(t *testing.T) {

	baseURL, err := normalizeBaseURL("https://example.local:8080/api/v1")
	require.NoError(t, err)

	candidates := buildSwaggerCandidates(baseURL)

	require.Contains(t, candidates, "https://example.local:3003/doc")
	require.Contains(t, candidates, "http://example.local:3003/doc")
	require.Contains(t, candidates, "https://example.local:8080/api/v1/doc")
}

func TestSystemPortBaseURLs_BackendAlreadyOnSystemPort(t *testing.T) {

	baseURL, err := normalizeBaseURL("http://example.local:3003/api")
	require.NoError(t, err)

	require.Empty(t, systemPortBaseURLs(baseURL))
}

func TestSystemPortBaseURLs_NoPort(t *testing.T) {

	baseURL, err := normalizeBaseURL("http://example.local/api")
	require.NoError(t, err)

	bases := systemPortBaseURLs(baseURL)
	require.Len(t, bases, 1)
	require.Equal(t, "http://example.local:3003/", bases[0].String())
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
