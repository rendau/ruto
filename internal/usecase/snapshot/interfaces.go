package snapshot

import (
	"context"

	appModel "github.com/rendau/ruto/internal/domain/app/model"
	endpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
	rootModel "github.com/rendau/ruto/internal/domain/root/model"
	snapshotModel "github.com/rendau/ruto/internal/domain/snapshot/model"
)

type ServiceI interface {
	GetVersion(ctx context.Context) (string, error)
	Get(ctx context.Context) (*snapshotModel.Snapshot, error)
	Set(ctx context.Context, obj *snapshotModel.Snapshot) error
}

type RootServiceI interface {
	Get(ctx context.Context) (*rootModel.Root, error)
}

type AppServiceI interface {
	List(ctx context.Context, pars *appModel.ListReq) ([]*appModel.App, int64, error)
}

type EndpointServiceI interface {
	List(ctx context.Context, pars *endpointModel.ListReq) ([]*endpointModel.Endpoint, int64, error)
}

type GatewaysNotifierI interface {
	NotifyAll()
}
