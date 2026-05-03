package root

import (
	"context"

	domModel "github.com/rendau/ruto/internal/domain/root/model"
)

type ServiceI interface {
	Get(ctx context.Context) (*domModel.Main, error)
	Set(ctx context.Context, obj *domModel.Edit) error
}
