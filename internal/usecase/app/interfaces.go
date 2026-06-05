package app

import (
	"context"

	"github.com/rendau/ruto/internal/domain/app/model"
	endpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
	rootModel "github.com/rendau/ruto/internal/domain/root/model"
	sessionModel "github.com/rendau/ruto/internal/domain/session/model"
	swaggerService "github.com/rendau/ruto/internal/service/swagger"
)

type ServiceI interface {
	List(ctx context.Context, pars *model.ListReq) ([]*model.App, int64, error)
	Get(ctx context.Context, id string, errNE bool) (*model.App, bool, error)
	Create(ctx context.Context, obj *model.App) (string, error)
	Update(ctx context.Context, id string, obj *model.App) error
	Delete(ctx context.Context, id string) error
}

type SessionServiceI interface {
	FromContext(ctx context.Context) *sessionModel.Session
	CtxIsAuthorized(ctx context.Context) bool
	CtxIsAdmin(ctx context.Context) bool
	CtxHasFullAppAccess(ctx context.Context) bool
	CtxGetAppIds(ctx context.Context) []string
}

type EndpointServiceI interface {
	List(ctx context.Context, pars *endpointModel.ListReq) ([]*endpointModel.Endpoint, int64, error)
}

type RootServiceI interface {
	Get(ctx context.Context) (*rootModel.Root, error)
}

type SwaggerServiceI interface {
	LoadEndpoints(ctx context.Context, swaggerURL string) ([]swaggerService.Endpoint, error)
}
