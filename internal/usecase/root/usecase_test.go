package root

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	authModel "github.com/rendau/ruto/internal/domain/auth/model"
	rootModel "github.com/rendau/ruto/internal/domain/root/model"
	sessionModel "github.com/rendau/ruto/internal/domain/session/model"
	varsModel "github.com/rendau/ruto/internal/domain/vars/model"
	"github.com/rendau/ruto/internal/errs"
)

type testSessionService struct {
	session *sessionModel.Session
}

type testRootService struct {
	item *rootModel.Root
	err  error
}

func (s *testRootService) Get(_ context.Context) (*rootModel.Root, error) {
	return s.item, s.err
}

func (s *testRootService) Set(_ context.Context, _ *rootModel.Root) error {
	return nil
}

func (s *testSessionService) FromContext(_ context.Context) *sessionModel.Session {
	return s.session
}

func TestUsecase_Interpolate(t *testing.T) {
	uc := New(&testRootService{
		item: &rootModel.Root{
			Variables: varsModel.Vars{
				"db_user": "postgres",
				"db_pass": "{{secret}}",
			},
			Auth: authModel.Auth{
				Methods: []*authModel.AuthMethod{
					{
						APIKey: &authModel.AuthMethodAPIKey{
							Keys: []string{"{{db_user}}", "{{db_pass}}", "{{api_key}}"},
						},
					},
				},
			},
		},
	}, &testSessionService{
		session: &sessionModel.Session{Id: 1},
	})

	item, err := uc.Interpolate(context.Background(), varsModel.Vars{
		"secret":  "qwerty",
		"api_key": "abc123",
		"db_user": "request-user",
	})
	require.NoError(t, err)
	require.Equal(t, varsModel.Vars{
		"secret":  "qwerty",
		"api_key": "abc123",
		"db_user": "request-user",
		"db_pass": "{{secret}}",
	}, item.Variables)
	require.Equal(t, []string{"request-user", "qwerty", "abc123"}, item.Auth.Methods[0].APIKey.Keys)
}

func TestUsecase_Interpolate_NotAuthorized(t *testing.T) {
	uc := New(&testRootService{}, &testSessionService{
		session: &sessionModel.Session{Id: 0},
	})

	_, err := uc.Interpolate(context.Background(), varsModel.Vars{
		"secret": "qwerty",
	})
	require.ErrorIs(t, err, errs.NotAuthorized)
}

func TestUsecase_GetJwtKidsByURLs_FilterRSAlgorithms(t *testing.T) {

	jwkServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"keys": [
				{"kid":"kid-rs256","alg":"RS256"},
				{"kid":"kid-rs512","alg":"RS512"},
				{"kid":"kid-rs1","alg":"RS1"},
				{"kid":"kid-es256","alg":"ES256"},
				{"kid":"kid-none","alg":""},
				{"kid":"kid-rs256","alg":"RS256"},
				{"kid":"","alg":"RS256"}
			]
		}`))
	}))
	defer jwkServer.Close()

	uc := New(nil, &testSessionService{
		session: &sessionModel.Session{Id: 1},
	})

	kids, err := uc.GetJwtKidsByURLs(context.Background(), []string{
		jwkServer.URL,
		"   " + jwkServer.URL + "   ",
		"",
	})
	require.NoError(t, err)
	require.Equal(t, []string{"kid-rs256", "kid-rs512"}, kids)
}

func TestUsecase_GetJwtKidsByURLs_NotAuthorized(t *testing.T) {

	uc := New(nil, &testSessionService{
		session: &sessionModel.Session{Id: 0},
	})

	_, err := uc.GetJwtKidsByURLs(context.Background(), []string{"https://example.com/jwks.json"})
	require.ErrorIs(t, err, errs.NotAuthorized)
}
