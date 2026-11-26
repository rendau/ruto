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

func (s *testSessionService) CtxIsAuthorized(_ context.Context) bool {
	return s.session.IsAuthorized()
}

func (s *testSessionService) CtxIsAdmin(_ context.Context) bool {
	return s.session.IsAdmin()
}

func TestUsecase_Get_RedactsForNonAdmin(t *testing.T) {
	rootObj := &rootModel.Root{
		BaseUrl: "https://api.example.com",
		Variables: varsModel.Vars{
			"BACKEND_TOKEN": "real-token",
		},
		Auth: authModel.Auth{
			Enabled: true,
			Methods: []*authModel.AuthMethod{
				{
					Basic: &authModel.AuthMethodBasic{
						Users: []authModel.AuthMethodBasicUser{{Username: "u", Password: "real-password"}},
					},
				},
				{
					APIKey: &authModel.AuthMethodAPIKey{
						Header: "X-Key",
						Keys:   []authModel.AuthMethodAPIKeyItem{{Name: "svc", Key: "real-key"}},
					},
				},
			},
		},
	}

	nonAdminUc := New(&testRootService{item: rootObj}, &testSessionService{
		session: &sessionModel.Session{Id: 2, AppIds: []string{"app-1"}},
	})
	got, err := nonAdminUc.Get(context.Background())
	require.NoError(t, err)
	require.Equal(t, "https://api.example.com", got.BaseUrl)
	require.Equal(t, varsModel.RedactedPlaceholder, got.Variables["BACKEND_TOKEN"])
	require.Equal(t, varsModel.RedactedPlaceholder, got.Auth.Methods[0].Basic.Users[0].Password)
	require.Equal(t, varsModel.RedactedPlaceholder, got.Auth.Methods[1].APIKey.Keys[0].Key)
	require.Equal(t, "svc", got.Auth.Methods[1].APIKey.Keys[0].Name)

	adminUc := New(&testRootService{item: rootObj}, &testSessionService{
		session: &sessionModel.Session{Id: 1, Admin: true},
	})
	gotAdmin, err := adminUc.Get(context.Background())
	require.NoError(t, err)
	require.Equal(t, "real-token", gotAdmin.Variables["BACKEND_TOKEN"])
	require.Equal(t, "real-password", gotAdmin.Auth.Methods[0].Basic.Users[0].Password)
	require.Equal(t, "real-key", gotAdmin.Auth.Methods[1].APIKey.Keys[0].Key)
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
							Keys: []authModel.AuthMethodAPIKeyItem{{Key: "{{db_user}}"}, {Key: "{{db_pass}}"}, {Key: "{{api_key}}"}},
						},
					},
				},
			},
		},
	}, &testSessionService{
		session: &sessionModel.Session{Id: 1, Admin: true},
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
	}, item.Variables)
	require.Equal(t, []authModel.AuthMethodAPIKeyItem{{Key: "request-user"}, {Key: "{{db_pass}}"}, {Key: "abc123"}}, item.Auth.Methods[0].APIKey.Keys)
}

func TestUsecase_Interpolate_RedactsForNonAdmin(t *testing.T) {
	uc := New(&testRootService{
		item: &rootModel.Root{
			Auth: authModel.Auth{
				Methods: []*authModel.AuthMethod{
					{
						APIKey: &authModel.AuthMethodAPIKey{
							Keys: []authModel.AuthMethodAPIKeyItem{{Key: "real-key"}},
						},
					},
				},
			},
		},
	}, &testSessionService{
		session: &sessionModel.Session{Id: 2, AppIds: []string{"app-1"}},
	})

	item, err := uc.Interpolate(context.Background(), nil)
	require.NoError(t, err)
	require.Equal(t, varsModel.RedactedPlaceholder, item.Auth.Methods[0].APIKey.Keys[0].Key)
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

func TestUsecase_GetJwtKidsByURLs_FilterAlgorithms(t *testing.T) {

	jwkServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"keys": [
				{"kid":"kid-rs256","kty":"RSA","alg":"RS256"},
				{"kid":"kid-rs512","kty":"RSA","alg":"RS512"},
				{"kid":"kid-rs1","kty":"RSA","alg":"RS1"},
				{"kid":"kid-hs256","kty":"oct","alg":"HS256"},
				{"kid":"kid-es256","kty":"EC","crv":"P-256","alg":"ES256"},
				{"kid":"kid-ec-noalg","kty":"EC","crv":"P-384"},
				{"kid":"kid-ed","kty":"OKP","crv":"Ed25519","alg":"EdDSA"},
				{"kid":"kid-none","kty":"RSA","alg":""},
				{"kid":"kid-rs256","kty":"RSA","alg":"RS256"},
				{"kid":"","kty":"RSA","alg":"RS256"}
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
	require.Equal(t, []string{"kid-ec-noalg", "kid-ed", "kid-es256", "kid-rs256", "kid-rs512"}, kids)
}

func TestUsecase_GetJwtKidsByURLs_NotAuthorized(t *testing.T) {

	uc := New(nil, &testSessionService{
		session: &sessionModel.Session{Id: 0},
	})

	_, err := uc.GetJwtKidsByURLs(context.Background(), []string{"https://example.com/jwks.json"})
	require.ErrorIs(t, err, errs.NotAuthorized)
}
