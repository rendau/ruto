package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/samber/lo"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/rendau/ruto/internal/constant"
	"github.com/rendau/ruto/pkg/proto/ruto_v1"

	"github.com/rendau/ruto/internal/config"

	handlerGrpcP "github.com/rendau/ruto/internal/handler/grpc"

	domainRootRepoDbP "github.com/rendau/ruto/internal/domain/root/repo/db"
	domainRootServiceP "github.com/rendau/ruto/internal/domain/root/service"
	usecaseRootP "github.com/rendau/ruto/internal/usecase/root"

	serviceGwP "github.com/rendau/ruto/internal/service/gw"
)

type App struct {
	globalTracerCloser io.Closer

	pgpool *pgxpool.Pool

	grpcServer *GrpcServer
	httpServer *http.Server
	gw         *serviceGwP.Service

	ctx       context.Context
	ctxCancel context.CancelFunc

	exitCode int
}

func (a *App) Init() {
	var err error

	a.ctx, a.ctxCancel = context.WithCancel(context.Background())

	// logger
	initLogger(config.Conf.Debug, config.Conf.LogLevel)

	// globalTracer
	{
		if config.Conf.WithTracing && config.Conf.JaegerAddress != "" {
			slog.Info("tracing enabled")
			_, a.globalTracerCloser, err = tracerInitGlobal(config.Conf.JaegerAddress, constant.ServiceName)
			errCheck(err, "tracerInitGlobal")
		}
	}

	if config.Conf.Debug {
		slog.Info("DEBUG mode enabled")
	}

	// pgpool
	a.pgpool, err = initPgPool(config.Conf.PgDsn)
	errCheck(err, "pgpool init")

	// migrations
	{
		runMigrations()
		slog.Info("PG-migrations have been successfully applied")
	}

	// root
	domainRootRepoDb := domainRootRepoDbP.New(a.pgpool)
	domainRootService := domainRootServiceP.New(domainRootRepoDb)
	usecaseRoot := usecaseRootP.New(domainRootService)
	handlerGrpcRoot := handlerGrpcP.NewRoot(usecaseRoot)

	// grpc server
	{
		a.grpcServer = NewGrpcServer("main", func(server *grpc.Server) {
			ruto_v1.RegisterRootServer(server, handlerGrpcRoot)
		})
	}

	// http-gw server
	{
		var handler http.Handler

		handler, err = GrpcGatewayCreateHandler(func(mux *runtime.ServeMux) error {
			opts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}

			var conn *grpc.ClientConn
			conn, err = grpc.NewClient("localhost:"+strconv.Itoa(config.Conf.GrpcPort), opts...)
			errCheck(err, "grpc.Dial")

			// register grpc handlers
			handlers := []func(context.Context, *runtime.ServeMux, *grpc.ClientConn) error{
				ruto_v1.RegisterRootHandler,
			}
			for _, h := range handlers {
				err = h(context.Background(), mux, conn)
				if err != nil {
					return fmt.Errorf("grpc-gateway: register grpc-handler: %w", err)
				}
			}

			return nil
		})
		errCheck(err, "grpcGatewayCreateHandler")

		// server
		a.httpServer = &http.Server{
			Addr:              ":" + strconv.Itoa(config.Conf.HttpPort),
			Handler:           handler,
			ReadHeaderTimeout: 2 * time.Second,
			ReadTimeout:       time.Minute,
			MaxHeaderBytes:    300 * 1024,
		}
	}

	// gw-server-http
	a.gw = serviceGwP.New(a.ctx, config.Conf.GwPort)

	// err = a.gw.SetConfig(&gwConfig.Root{
	// 	PublicBaseUrl: "https://example.com",
	// 	Cors:          gwConfig.RootCors{},
	// 	Jwt: []*gwConfig.RootJwt{
	// 		{JwkUrl: "https://api.mdev.kz/jwts/jwk/set"},
	// 	},
	// 	Apps: []*gwConfig.App{
	// 		{
	// 			PublicPathPrefix: "/ep",
	// 			Backend: gwConfig.AppBackend{
	// 				UrlStr: "https://api.mdev.kz/ep",
	// 			},
	// 			Endpoints: []*gwConfig.Endpoint{
	// 				{
	// 					Method:        "GET",
	// 					Path:          "dict",
	// 					Backend:       gwConfig.EndpointBackend{},
	// 					JwtValidation: gwConfig.EndpointJwtValidation{},
	// 					IpValidation:  gwConfig.EndpointIpValidation{},
	// 				},
	// 			},
	// 		},
	// 	},
	// })
	// errCheck(err, "gw-server SetConfig")
}

func (a *App) PreStartHook() {
	// slog.Info("PreStartHook")
}

func (a *App) Start() {
	slog.Info("Starting")

	// grpc server
	{
		err := a.grpcServer.Start()
		errCheck(err, "grpcServer.Start")
	}

	// http-gw server
	{
		go func() {
			err := a.httpServer.ListenAndServe()
			if err != nil && !errors.Is(err, http.ErrServerClosed) {
				// errCheck(err, "http-server stopped")
			}
		}()
		slog.Info("http-server started " + a.httpServer.Addr)
	}

	// gw-server-http
	a.gw.Start()
}

func (a *App) Listen() {
	signalCtx, signalCtxCancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer signalCtxCancel()

	// wait signal
	<-signalCtx.Done()
}

func (a *App) Stop() {
	slog.Info("Shutting down...")

	// stop context
	a.ctxCancel()

	// gw-server-http
	{
		err := a.gw.Stop(time.Minute)
		if err != nil {
			slog.Error("http-server Stop error", "error", err)
			a.exitCode = 1
		}
	}

	// http-gw server
	{
		ctx, ctxCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer ctxCancel()

		if err := a.httpServer.Shutdown(ctx); err != nil {
			slog.Error("http-server shutdown error", "error", err)
			a.exitCode = 1
		}
	}

	// grpc server
	a.grpcServer.Stop()
}

func (a *App) WaitJobs() {
	slog.Info("waiting jobs")
}

func (a *App) Exit() {
	slog.Info("Exit")

	if a.globalTracerCloser != nil {
		_ = a.globalTracerCloser.Close()
	}

	a.pgpool.Close()

	os.Exit(a.exitCode)
}

func errCheck(err error, msg string) {
	if err != nil {
		if msg != "" {
			err = fmt.Errorf("%s: %w", msg, err)
		}
		slog.Error(err.Error())
		os.Exit(1)
	}
}
