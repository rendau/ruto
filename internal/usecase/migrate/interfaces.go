package migrate

import (
	"context"

	sessionModel "github.com/rendau/ruto/internal/domain/session/model"
	serviceMigrate "github.com/rendau/ruto/internal/service/migrate"
)

type ServiceI interface {
	Run(ctx context.Context, realmName, jwkURL string) (*serviceMigrate.Result, error)
}

type SessionServiceI interface {
	FromContext(ctx context.Context) *sessionModel.Session
}
