package grpc

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"time"

	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/rendau/ruto/internal/handler/grpc/dto"
	usecase "github.com/rendau/ruto/internal/usecase/snapshot"
	"github.com/rendau/ruto/pkg/proto/ruto_v1"
)

type Snapshot struct {
	ruto_v1.UnsafeSnapshotServer
	usecase *usecase.Usecase
}

func NewSnapshot(usecase *usecase.Usecase) *Snapshot {
	return &Snapshot{usecase: usecase}
}

func (h *Snapshot) Get(ctx context.Context, _ *emptypb.Empty) (*ruto_v1.SnapshotResponse, error) {
	result, err := h.usecase.Get(ctx)
	if err != nil {
		return nil, err
	}
	return &ruto_v1.SnapshotResponse{
		Data: dto.JsonObjToGrpcStruct(result),
	}, nil
}

func (h *Snapshot) SubscribeVersions(_ *emptypb.Empty, stream ruto_v1.Snapshot_SubscribeVersionsServer) error {
	ctx := stream.Context()

	currentVersion, err := h.getVersion(ctx)
	if err != nil {
		return err
	}
	if err = stream.Send(&ruto_v1.SnapshotVersion{Version: currentVersion}); err != nil {
		return err
	}

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			nextVersion, getErr := h.getVersion(ctx)
			if getErr != nil {
				slog.Error("snapshot subscribe: getVersion", "error", getErr)
				continue
			}
			if nextVersion == currentVersion {
				continue
			}
			currentVersion = nextVersion
			if err = stream.Send(&ruto_v1.SnapshotVersion{Version: currentVersion}); err != nil {
				return err
			}
		}
	}
}

func (h *Snapshot) getVersion(ctx context.Context) (string, error) {
	result, err := h.usecase.Get(ctx)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(result)
	return hex.EncodeToString(sum[:]), nil
}
