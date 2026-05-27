package gw

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/samber/lo"

	rootModel "github.com/rendau/ruto/internal/domain/root/model"
	"github.com/rendau/ruto/internal/service/gw/core_client"
	handlerHttp "github.com/rendau/ruto/internal/service/gw/handler/http"
	"github.com/rendau/ruto/internal/service/gw/jwk"
	localHttp "github.com/rendau/ruto/internal/service/gw/server/http"
)

type Service struct {
	globalCtx   context.Context
	server      *localHttp.Service
	jwk         *jwk.Service
	coreClient  *core_client.Service
	logRequests bool
	ready       atomic.Bool
}

func New(globalCtx context.Context, serverPort int, configAddress string, logRequests bool) (*Service, error) {
	var err error

	service := &Service{
		globalCtx:   globalCtx,
		server:      localHttp.New(serverPort),
		jwk:         jwk.New(globalCtx),
		logRequests: logRequests,
	}
	service.ready.Store(false)

	service.coreClient, err = core_client.New(globalCtx, configAddress, service.SetConfig)
	if err != nil {
		return nil, fmt.Errorf("config_client.New: %w", err)
	}

	return service, nil
}

func (s *Service) Start() {
	s.jwk.Start()
	s.coreClient.Start()
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
	httpHandler, err := handlerHttp.New(conf, s.jwk, s.logRequests)
	if err != nil {
		return fmt.Errorf("handlerHttp.New: %w", err)
	}
	s.server.SetHandler(httpHandler)

	s.ready.Store(true)

	// set jwk URLs
	jwkUrls := lo.Map(conf.Jwt, func(item rootModel.RootJwt, _ int) string {
		return item.JwkUrl
	})
	s.jwk.SetUrls(jwkUrls)

	return nil
}

func (s *Service) IsReady() bool {
	return s.ready.Load()
}
