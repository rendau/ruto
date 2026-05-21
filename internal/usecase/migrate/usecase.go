package migrate

import (
	"context"
	"fmt"
	"strings"

	"github.com/rendau/ruto/internal/errs"
	serviceMigrate "github.com/rendau/ruto/internal/service/migrate"
)

type Usecase struct {
	svc        ServiceI
	sessionSvc SessionServiceI
}

func New(svc ServiceI, sessionSvc SessionServiceI) *Usecase {
	return &Usecase{
		svc:        svc,
		sessionSvc: sessionSvc,
	}
}

func (u *Usecase) Run(ctx context.Context, req *RunReq) (*serviceMigrate.Result, error) {
	extractedSession := u.sessionSvc.FromContext(ctx)
	if extractedSession.Id == 0 {
		return nil, errs.NotAuthorized
	}
	if !extractedSession.IsAdmin {
		return nil, errs.NoPermission
	}
	if req == nil {
		return nil, errs.InvalidRequest
	}

	realmName := strings.TrimSpace(req.RealmName)
	if realmName == "" {
		return nil, fmt.Errorf("realm_name: empty")
	}
	jwkURL := strings.TrimSpace(req.JwkURL)

	result, err := u.svc.Run(ctx, realmName, jwkURL)
	if err != nil {
		return nil, fmt.Errorf("svc.Run: %w", err)
	}
	return result, nil
}

type RunReq struct {
	RealmName string
	JwkURL    string
}
