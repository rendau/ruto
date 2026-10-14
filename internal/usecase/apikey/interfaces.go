package apikey

import (
	"context"

	"github.com/rendau/ruto/internal/domain/apikey/model"
	sessionModel "github.com/rendau/ruto/internal/domain/session/model"
	usrModel "github.com/rendau/ruto/internal/domain/usr/model"
)

type ServiceI interface {
	List(ctx context.Context, pars *model.ListReq) ([]*model.ApiKey, int64, error)
	Get(ctx context.Context, id string, errNE bool) (*model.ApiKey, bool, error)
	GetByKey(ctx context.Context, key string) (*model.ApiKey, bool, error)
	Create(ctx context.Context, obj *model.ApiKey) (string, error)
	Delete(ctx context.Context, id string) error
	TouchLastUsed(ctx context.Context, id string) error
}

type UsrServiceI interface {
	Get(ctx context.Context, id int64, errNE bool) (*usrModel.Usr, bool, error)
}

type SessionServiceI interface {
	FromContext(ctx context.Context) *sessionModel.Session
}
