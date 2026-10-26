package system

import (
	"context"
	"time"

	changeRequestModel "github.com/rendau/ruto/internal/domain/changerequest/model"
	usecaseSystem "github.com/rendau/ruto/internal/usecase/system"
)

type UsecaseI interface {
	Gateways(ctx context.Context) (*usecaseSystem.Gateways, error)
	HasUnpublishedChanges(ctx context.Context) (bool, error)
	PendingChangeRequests(ctx context.Context, stuckAfter time.Duration) (*changeRequestModel.PendingStats, error)
	RoutesVersion(ctx context.Context) (string, error)
	Routes(ctx context.Context) (*usecaseSystem.Routes, error)
}
