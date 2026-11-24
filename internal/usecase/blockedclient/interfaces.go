package blockedclient

import (
	"context"
	"time"

	"github.com/rendau/ruto/internal/domain/blockedclient/model"
	rootModel "github.com/rendau/ruto/internal/domain/root/model"
	sessionModel "github.com/rendau/ruto/internal/domain/session/model"
	usrModel "github.com/rendau/ruto/internal/domain/usr/model"
)

type ServiceI interface {
	List(ctx context.Context, pars *model.ListReq) ([]*model.BlockedClient, int64, error)
	Get(ctx context.Context, id string, errNE bool) (*model.BlockedClient, bool, error)
	GetNotRemoved(ctx context.Context, ip string) (*model.BlockedClient, bool, error)
	CountActive(ctx context.Context, now time.Time, source *string) (int64, error)
	CountCreatedSince(ctx context.Context, source string, since time.Time) (int64, error)
	ListLiftedByAdmin(ctx context.Context, since time.Time) ([]*model.BlockedClient, error)
	RemoveMany(ctx context.Context, req *model.RemoveMany) (int64, error)
	Create(ctx context.Context, obj *model.BlockedClient) (string, error)
	Update(ctx context.Context, id string, obj *model.Edit) error
	Lock(ctx context.Context) error
	CloseExpired(ctx context.Context, ip string, now time.Time) error
	Active(ctx context.Context, now time.Time) ([]*model.Rule, string, error)
	Prune(ctx context.Context, endedBefore time.Time) (int64, error)
}

type TransactionManagerI interface {
	TxFn(ctx context.Context, f func(context.Context) error) error
}

type SessionServiceI interface {
	FromContext(ctx context.Context) *sessionModel.Session
}

type RootServiceI interface {
	Get(ctx context.Context) (*rootModel.Root, error)
}

type UsrServiceI interface {
	Get(ctx context.Context, id int64, errNE bool) (*usrModel.Usr, bool, error)
}

// GatewaysNotifierI pushes "check_blocklist" to every subscribed gateway.
type GatewaysNotifierI interface {
	NotifyBlocklist()
}
