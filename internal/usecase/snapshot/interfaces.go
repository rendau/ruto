package snapshot

import (
	"context"

	appModel "github.com/rendau/ruto/internal/domain/app/model"
	endpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
	rootModel "github.com/rendau/ruto/internal/domain/root/model"
)

type RootServiceI interface {
	Get(ctx context.Context) (*rootModel.Root, error)
}

type AppServiceI interface {
	List(ctx context.Context, pars *appModel.ListReq) ([]*appModel.App, int64, error)
}

type EndpointServiceI interface {
	List(ctx context.Context, pars *endpointModel.ListReq) ([]*endpointModel.Endpoint, int64, error)
}
