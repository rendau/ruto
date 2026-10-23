package gw

import (
	"context"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/samber/lo"

	rootModel "github.com/rendau/ruto/internal/domain/root/model"
	"github.com/rendau/ruto/internal/service/gw/core_client"
	handlerGrpc "github.com/rendau/ruto/internal/service/gw/handler/grpc"
	handlerHttp "github.com/rendau/ruto/internal/service/gw/handler/http"
	"github.com/rendau/ruto/internal/service/gw/handler/http/proxy"
	localGrpc "github.com/rendau/ruto/internal/service/gw/server/grpc"
	localHttp "github.com/rendau/ruto/internal/service/gw/server/http"
	"github.com/rendau/ruto/internal/service/gw/service/cache"
	"github.com/rendau/ruto/internal/service/gw/service/jwk"
)

type Service struct {
	globalCtx     context.Context
	httpServer    *localHttp.Service
	httpTransport *http.Transport
	cache         *cache.Service
	cacheSweep    time.Duration
	grpcServer    *localGrpc.Service
	coreClient    *core_client.Service
	ready         atomic.Bool
}

func New(
	globalCtx context.Context,
	httpPort, grpcPort int,
	configAddress string,
	backendIdleConnTimeout time.Duration,
	cacheMaxSize int64,
	cacheSweepInterval time.Duration,
) (*Service, error) {
	var err error

	service := &Service{
		globalCtx:     globalCtx,
		httpServer:    localHttp.New(httpPort),
		httpTransport: proxy.NewTransport(backendIdleConnTimeout),
		cache:         cache.New(cacheMaxSize),
		cacheSweep:    cacheSweepInterval,
	}
	if grpcPort > 0 {
		service.grpcServer = localGrpc.New(grpcPort)
	}
	service.ready.Store(false)

	service.coreClient, err = core_client.New(globalCtx, configAddress, service.SetConfig)
	if err != nil {
		return nil, fmt.Errorf("config_client.New: %w", err)
	}

	return service, nil
}

func (s *Service) Start() {
	s.coreClient.Start()
	go s.cache.Run(s.globalCtx, s.cacheSweep)
	s.httpServer.Start()
	if s.grpcServer != nil {
		s.grpcServer.Start()
	}
}

func (s *Service) Stop(timeout time.Duration) error {
	if err := s.httpServer.Stop(timeout); err != nil {
		return err
	}
	if s.grpcServer != nil {
		if err := s.grpcServer.Stop(timeout); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) SetConfig(conf *rootModel.Root) error {
	// prepare config
	{
		// normalize
		err := conf.Normalize()
		if err != nil {
			return fmt.Errorf("config normalize: %w", err)
		}

		// inherit
		conf.InheritDown()

		// interpolate
		conf.Interpolate()
	}

	// set http handler
	httpHandler, err := handlerHttp.New(conf, s.httpTransport, s.cache)
	if err != nil {
		return fmt.Errorf("handlerHttp.New: %w", err)
	}
	s.httpServer.SetHandler(httpHandler)

	if s.grpcServer != nil {
		grpcHandler, grpcErr := handlerGrpc.New(conf)
		if grpcErr != nil {
			return fmt.Errorf("handlerGrpc.New: %w", grpcErr)
		}
		s.grpcServer.SetHandler(grpcHandler)
	}

	s.ready.Store(true)

	// set jwk URLs
	jwkUrls := lo.Map(conf.Jwt, func(item rootModel.RootJwt, _ int) string {
		return item.JwkUrl
	})
	jwk.Ins().SetUrls(jwkUrls)

	return nil
}

func (s *Service) IsReady() bool {
	return s.ready.Load()
}
