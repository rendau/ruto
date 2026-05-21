package grpc

import (
	"context"

	usecase "github.com/rendau/ruto/internal/usecase/migrate"
	"github.com/rendau/ruto/pkg/proto/ruto_v1"
)

type Migrate struct {
	ruto_v1.UnsafeMigrateServer
	usecase *usecase.Usecase
}

func NewMigrate(usecase *usecase.Usecase) *Migrate {
	return &Migrate{usecase: usecase}
}

func (h *Migrate) Run(ctx context.Context, req *ruto_v1.MigrateRunReq) (*ruto_v1.MigrateRunRep, error) {
	result, err := h.usecase.Run(ctx, &usecase.RunReq{
		RealmName: req.RealmName,
		JwkURL:    req.JwkUrl,
	})
	if err != nil {
		return nil, err
	}

	return &ruto_v1.MigrateRunRep{
		RealmName:     result.RealmName,
		RootBaseUrl:   result.RootBaseURL,
		AppCount:      result.AppCount,
		EndpointCount: result.EndpointCount,
	}, nil
}
