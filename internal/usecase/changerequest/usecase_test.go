package changerequest

import (
	"context"
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

// Admin-only fields (allow_wildcard, allow_trailing_slash) are checked by the
// usecases under the applier's session, so only a human admin may apply.
func TestUsecase_Apply_AdminOnly(t *testing.T) {
	sessions := map[string]*sessionModel.Session{
		"user with full app access": {Id: 1, AllApps: true},
		"admin api key":             {Id: 2, Admin: true, ApiKeyId: "key-1"},
	}

	for name, session := range sessions {
		t.Run(name, func(t *testing.T) {
			uc := &Usecase{sessionSvc: &testSessionService{session: session}}

			require.ErrorIs(t, uc.Apply(context.Background(), 1, false), errs.NoPermission)
		})
	}
}
