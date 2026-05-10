package root

import (
	"context"

	"github.com/rendau/ruto/internal/domain/root/model"
)

type ServiceI interface {
	Get(ctx context.Context) (*model.Root, error)
	Set(ctx context.Context, obj *model.Root) error
}
