package endpoint

import (
	"context"

	appModel "github.com/rendau/ruto/internal/domain/app/model"
	"github.com/rendau/ruto/internal/domain/endpoint/model"
	rootModel "github.com/rendau/ruto/internal/domain/root/model"
	sessionModel "github.com/rendau/ruto/internal/domain/session/model"
)

type ServiceI interface {
	List(ctx context.Context, pars *model.ListReq) ([]*model.Endpoint, int64, error)
	Get(ctx context.Context, id string, errNE bool) (*model.Endpoint, bool, error)
	Create(ctx context.Context, obj *model.Endpoint) (string, error)
	Update(ctx context.Context, id string, obj *model.Endpoint) error
	Delete(ctx context.Context, id string) error
}

type SessionServiceI interface {
	FromContext(ctx context.Context) *sessionModel.Session
}

type RootServiceI interface {
	Get(ctx context.Context) (*rootModel.Root, error)
}

type AppServiceI interface {
	Get(ctx context.Context, id string, errNE bool) (*appModel.App, bool, error)
}
