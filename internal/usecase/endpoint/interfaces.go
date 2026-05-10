package endpoint

import (
	"context"

	"github.com/rendau/ruto/internal/domain/endpoint/model"
)

type ServiceI interface {
	List(ctx context.Context, pars *model.ListReq) ([]*model.Main, int64, error)
	Get(ctx context.Context, id string, errNE bool) (*model.Main, bool, error)
	Create(ctx context.Context, obj *model.Edit) (string, error)
	Update(ctx context.Context, id string, obj *model.Edit) error
	Delete(ctx context.Context, id string) error
}
