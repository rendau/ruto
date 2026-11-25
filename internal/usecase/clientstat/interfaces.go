package clientstat

import (
	"context"
	"time"

	blockedClientModel "github.com/rendau/ruto/internal/domain/blockedclient/model"
	"github.com/rendau/ruto/internal/domain/clientstat/model"
	sessionModel "github.com/rendau/ruto/internal/domain/session/model"
)

type ServiceI interface {
	Add(ctx context.Context, receivedAt time.Time, items []*model.ClientStat) error
	Overview(ctx context.Context, w *model.Window) (*model.Overview, error)
	Series(ctx context.Context, w *model.Window, ip string) (*model.Series, error)
	Top(ctx context.Context, req *model.TopReq) ([]*model.Item, error)
	DeleteBefore(ctx context.Context, before time.Time) (int64, error)
}

type BlockedClientServiceI interface {
	List(ctx context.Context, pars *blockedClientModel.ListReq) ([]*blockedClientModel.BlockedClient, int64, error)
}

type SessionServiceI interface {
	FromContext(ctx context.Context) *sessionModel.Session
}
