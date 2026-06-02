package root

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	sessionModel "github.com/rendau/ruto/internal/domain/session/model"
	"github.com/rendau/ruto/internal/errs"
)

type testSessionService struct {
	session *sessionModel.Session
}

func (s *testSessionService) FromContext(_ context.Context) *sessionModel.Session {
	return s.session
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
