package endpoint

import (
	"context"

	"github.com/rendau/ruto/internal/domain/endpoint/model"
)

type ServiceI interface {
	List(ctx context.Context, pars *model.ListReq) ([]*model.Endpoint, int64, error)
	Get(ctx context.Context, id string, errNE bool) (*model.Endpoint, bool, error)
	Create(ctx context.Context, obj *model.Endpoint) (string, error)
	Update(ctx context.Context, id string, obj *model.Endpoint) error
	Delete(ctx context.Context, id string) error
}
