package system

import (
	"context"
	"time"

	changeRequestModel "github.com/rendau/ruto/internal/domain/changerequest/model"
	snapshotModel "github.com/rendau/ruto/internal/domain/snapshot/model"
	usecaseGateway "github.com/rendau/ruto/internal/usecase/gateway"
)

type SnapshotServiceI interface {
	GetVersion(ctx context.Context) (string, error)
	Get(ctx context.Context) (*snapshotModel.Snapshot, error)
}

type SnapshotUsecaseI interface {
	CurrentVersion(ctx context.Context) (string, error)
}

type GatewayUsecaseI interface {
	ListAll(ctx context.Context) ([]*usecaseGateway.Item, error)
}

type GatewaysPoolI interface {
	ConnectedIDs() []string
}

type ChangeRequestServiceI interface {
	PendingStats(ctx context.Context, stuckAfter time.Duration) (*changeRequestModel.PendingStats, error)
}
