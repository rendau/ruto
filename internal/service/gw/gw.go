package gw

import (
	"context"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	rootModel "github.com/rendau/ruto/internal/domain/root/model"
	"github.com/rendau/ruto/internal/service/gw/core_client"
	handlerGrpc "github.com/rendau/ruto/internal/service/gw/handler/grpc"
	handlerHttp "github.com/rendau/ruto/internal/service/gw/handler/http"
	"github.com/rendau/ruto/internal/service/gw/handler/http/proxy"
	localGrpc "github.com/rendau/ruto/internal/service/gw/server/grpc"
	localHttp "github.com/rendau/ruto/internal/service/gw/server/http"
	"github.com/rendau/ruto/internal/service/gw/service/cache"
	"github.com/rendau/ruto/internal/service/gw/service/clientip"
	"github.com/rendau/ruto/internal/service/gw/service/jwk"
	"github.com/rendau/ruto/internal/service/gw/service/ratelimit"
)

const rateLimitSweepInterval = 30 * time.Second

type Service struct {
	globalCtx     context.Context
	httpServer    *localHttp.Service
	httpTransport *http.Transport
	cache         *cache.Service
	cacheSweep    time.Duration
	rateLimits    *ratelimit.Store
	limits        *ratelimit.Service
	grpcServer    *localGrpc.Service
	grpcConns     *handlerGrpc.Conns
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
	rateLimitMaxKeys int,
	trustedProxyAddresses []string,
) (*Service, error) {
	var err error

	rateLimits := ratelimit.New(rateLimitMaxKeys)

	service := &Service{
		globalCtx:     globalCtx,
		httpServer:    localHttp.New(httpPort),
		httpTransport: proxy.NewTransport(backendIdleConnTimeout),
		cache:         cache.New(cacheMaxSize),
		cacheSweep:    cacheSweepInterval,
		rateLimits:    rateLimits,
		limits:        ratelimit.NewService(rateLimits, clientip.New(trustedProxyAddresses)),
	}
	if grpcPort > 0 {
		service.grpcServer = localGrpc.New(grpcPort)
		service.grpcConns = handlerGrpc.NewConns()
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
	go s.rateLimits.Run(s.globalCtx, rateLimitSweepInterval)
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
		s.grpcConns.Close()
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
	httpHandler, err := handlerHttp.New(conf, s.httpTransport, s.cache, s.limits)
	if err != nil {
		return fmt.Errorf("handlerHttp.New: %w", err)
	}
	s.httpServer.SetHandler(httpHandler)

	if s.grpcServer != nil {
		grpcHandler, grpcErr := handlerGrpc.New(conf, s.grpcConns, s.limits)
		if grpcErr != nil {
			return fmt.Errorf("handlerGrpc.New: %w", grpcErr)
		}
		s.grpcServer.SetHandler(grpcHandler)
		// Once the new handler serves, the backends that left the config are
		// let go; the ones that stayed keep their connections.
		s.grpcConns.Retain(grpcHandler.Targets())
	}

	s.ready.Store(true)

	jwk.Ins().SetProviders(conf.Jwt)

	return nil
}

func (s *Service) IsReady() bool {
	return s.ready.Load()
}
