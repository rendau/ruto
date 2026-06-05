package usr

import (
	"context"

	sessionModel "github.com/rendau/ruto/internal/domain/session/model"
	"github.com/rendau/ruto/internal/domain/usr/model"
)

type ServiceI interface {
	List(ctx context.Context, pars *model.ListReq) ([]*model.Usr, int64, error)
	Get(ctx context.Context, id int64, errNE bool) (*model.Usr, bool, error)
	AuthByUsernamePassword(ctx context.Context, username, password string) (*model.Usr, bool, error)
	HasAny(ctx context.Context) (bool, error)
	Create(ctx context.Context, obj *model.Edit) (int64, error)
	Update(ctx context.Context, id int64, obj *model.Edit) error
	Delete(ctx context.Context, id int64) error
}

type SessionServiceI interface {
	FromContext(ctx context.Context) *sessionModel.Session
	CtxIsAuthorized(ctx context.Context) bool
	CtxIsAdmin(ctx context.Context) bool
	CreateToken(usrId int64, isAdmin bool, allApps bool, appIds []string) (string, error)
}
