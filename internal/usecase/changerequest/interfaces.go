package changerequest

import (
	"context"
	"time"

	appModel "github.com/rendau/ruto/internal/domain/app/model"
	"github.com/rendau/ruto/internal/domain/changerequest/model"
	endpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
	rootModel "github.com/rendau/ruto/internal/domain/root/model"
	seenPathModel "github.com/rendau/ruto/internal/domain/seenpath/model"
	sessionModel "github.com/rendau/ruto/internal/domain/session/model"
	usrModel "github.com/rendau/ruto/internal/domain/usr/model"
)

type ServiceI interface {
	List(ctx context.Context, pars *model.ListReq) ([]*model.ChangeRequest, int64, error)
	Get(ctx context.Context, id int64, errNE bool) (*model.ChangeRequest, bool, error)
	GetLocked(ctx context.Context, id int64) (*model.ChangeRequest, error)
	Create(ctx context.Context, obj *model.ChangeRequest) (int64, error)
	Update(ctx context.Context, id int64, obj *model.Edit) error
	Cleanup(ctx context.Context, pendingTTL, retention time.Duration) (int64, int64, error)
}

type TransactionManagerI interface {
	TxFn(ctx context.Context, f func(context.Context) error) error
}

type SessionServiceI interface {
	FromContext(ctx context.Context) *sessionModel.Session
}

// Writes go through the usecases so a request is checked exactly like the
// same edit made in the admin UI.

type RootUsecaseI interface {
	Set(ctx context.Context, obj *rootModel.Root) error
}

type AppUsecaseI interface {
	Create(ctx context.Context, obj *appModel.App) (string, error)
	Update(ctx context.Context, id string, obj *appModel.App) error
	Delete(ctx context.Context, id string) error
}

type EndpointUsecaseI interface {
	Create(ctx context.Context, obj *endpointModel.Endpoint) (string, error)
	Update(ctx context.Context, id string, obj *endpointModel.Endpoint) error
	Delete(ctx context.Context, id string) error
}

type SeenPathUsecaseI interface {
	Delete(ctx context.Context, appId string, keys []*seenPathModel.SeenPathKey) error
}

// Reads go to the services: secrets must be resolvable regardless of what
// the caller may see.

type RootServiceI interface {
	Get(ctx context.Context) (*rootModel.Root, error)
}

type AppServiceI interface {
	List(ctx context.Context, pars *appModel.ListReq) ([]*appModel.App, int64, error)
	Get(ctx context.Context, id string, errNE bool) (*appModel.App, bool, error)
}

type EndpointServiceI interface {
	List(ctx context.Context, pars *endpointModel.ListReq) ([]*endpointModel.Endpoint, int64, error)
	Get(ctx context.Context, id string, errNE bool) (*endpointModel.Endpoint, bool, error)
}

type SeenPathServiceI interface {
	ListByPaths(ctx context.Context, appId string, keys []*seenPathModel.PathKey) ([]*seenPathModel.SeenPath, error)
}

type SnapshotUsecaseI interface {
	Validate(ctx context.Context) error
	Refresh(ctx context.Context) error
}

type UsrServiceI interface {
	Get(ctx context.Context, id int64, errNE bool) (*usrModel.Usr, bool, error)
}
