package gw

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/samber/lo"

	rootModel "github.com/rendau/ruto/internal/domain/root/model"
	"github.com/rendau/ruto/internal/service/gw/client"
	handlerHttp "github.com/rendau/ruto/internal/service/gw/handler/http"
	"github.com/rendau/ruto/internal/service/gw/jwk"
	localHttp "github.com/rendau/ruto/internal/service/gw/server/http"
)

type Service struct {
	globalCtx context.Context
	server    *localHttp.Service
	jwk       *jwk.Service
	client    *client.Service
}

func New(globalCtx context.Context, serverPort int, snapshotGrpcAddress string) *Service {
	srv := &Service{
		globalCtx: globalCtx,
		server:    localHttp.New(serverPort),
		jwk:       jwk.New(globalCtx),
	}
	srv.client = client.New(globalCtx, snapshotGrpcAddress, func(conf *rootModel.Root) error {
		if err := srv.SetConfig(conf); err != nil {
			slog.Error("gw snapshot-client: apply config", "error", err)
			return err
		}
		slog.Info("gw config updated", "apps_count", len(conf.Apps))
		return nil
	})
	return srv
}

func (s *Service) Start() {
	s.jwk.Start()
	s.client.Start()
	s.server.Start()
}

func (s *Service) Stop(timeout time.Duration) error {
	return s.server.Stop(timeout)
}

func (s *Service) SetConfig(conf *rootModel.Root) error {
	err := conf.Normalize()
	if err != nil {
		return fmt.Errorf("config normalize: %w", err)
	}

	// set http handler
	httpHandler, err := handlerHttp.New(conf)
	if err != nil {
		return fmt.Errorf("handlerHttp.New: %w", err)
	}
	s.server.SetHandler(httpHandler)

	// set jwk URLs
	jwkUrls := lo.Map(conf.Jwt, func(item rootModel.RootJwt, _ int) string {
		return item.JwkUrl
	})
	s.jwk.SetUrls(jwkUrls)

	return nil
}
