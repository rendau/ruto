package service

import (
	"context"
	"time"

	"github.com/rendau/ruto/internal/domain/changerequest/model"
)

type RepoDbI interface {
	List(ctx context.Context, pars *model.ListReq) ([]*model.ChangeRequest, int64, error)
	Get(ctx context.Context, id int64) (*model.ChangeRequest, bool, error)
	Lock(ctx context.Context, id int64) error
	Create(ctx context.Context, obj *model.ChangeRequest) (int64, error)
	Update(ctx context.Context, id int64, obj *model.Edit) error
	ExpirePending(ctx context.Context, createdBefore time.Time) (int64, error)
	DeleteClosed(ctx context.Context, decidedBefore time.Time) (int64, error)
}
