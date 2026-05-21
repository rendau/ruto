package app

import (
	// stdlib
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

	// third-party
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/samber/lo"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	// app-level constants and API
	"github.com/rendau/ruto/internal/constant"
	"github.com/rendau/ruto/pkg/proto/ruto_v1"

	// config
	"github.com/rendau/ruto/internal/config"

	// grpc handlers
	handlerGrpcP "github.com/rendau/ruto/internal/handler/grpc"

	// root module
	domainRootRepoDbP "github.com/rendau/ruto/internal/domain/root/repo/db"
	domainRootServiceP "github.com/rendau/ruto/internal/domain/root/service"
	usecaseRootP "github.com/rendau/ruto/internal/usecase/root"

	// app module
	domainAppRepoDbP "github.com/rendau/ruto/internal/domain/app/repo/db"
	domainAppServiceP "github.com/rendau/ruto/internal/domain/app/service"
	usecaseAppP "github.com/rendau/ruto/internal/usecase/app"

	// endpoint module
	domainEndpointRepoDbP "github.com/rendau/ruto/internal/domain/endpoint/repo/db"
	domainEndpointServiceP "github.com/rendau/ruto/internal/domain/endpoint/service"
	usecaseEndpointP "github.com/rendau/ruto/internal/usecase/endpoint"

	// user module
	domainUsrRepoDbP "github.com/rendau/ruto/internal/domain/usr/repo/db"
	domainUsrServiceP "github.com/rendau/ruto/internal/domain/usr/service"
	usecaseUsrP "github.com/rendau/ruto/internal/usecase/usr"

	// session module
	domainSessionServiceP "github.com/rendau/ruto/internal/domain/session/service"

	// gateway/snapshot/stats modules
	serviceGwP "github.com/rendau/ruto/internal/service/gw"
	serviceSnapshotP "github.com/rendau/ruto/internal/service/snapshot"
	usecaseSnapshotP "github.com/rendau/ruto/internal/usecase/snapshot"
	usecaseStatsP "github.com/rendau/ruto/internal/usecase/stats"

	// migrate module
	serviceMigrateP "github.com/rendau/ruto/internal/service/migrate"
	usecaseMigrateP "github.com/rendau/ruto/internal/usecase/migrate"
)

type App struct {
	globalTracerCloser io.Closer

	pgpool *pgxpool.Pool

	grpcServer *GrpcServer
	httpServer *http.Server

	gw *serviceGwP.Service

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

	// session
	sessionService := domainSessionServiceP.New(config.Conf.AdminJWTSecret)

	// root
	domainRootRepoDb := domainRootRepoDbP.New(a.pgpool)
	domainRootService := domainRootServiceP.New(domainRootRepoDb)
	usecaseRoot := usecaseRootP.New(domainRootService, sessionService)
	handlerGrpcRoot := handlerGrpcP.NewRoot(usecaseRoot)

	// app
	domainAppRepoDb := domainAppRepoDbP.New(a.pgpool)
	domainAppService := domainAppServiceP.New(domainAppRepoDb)
	usecaseApp := usecaseAppP.New(domainAppService, sessionService)
	handlerGrpcApp := handlerGrpcP.NewApp(usecaseApp)

	// endpoint
	domainEndpointRepoDb := domainEndpointRepoDbP.New(a.pgpool)
	domainEndpointService := domainEndpointServiceP.New(domainEndpointRepoDb)
	usecaseEndpoint := usecaseEndpointP.New(domainEndpointService, sessionService)
	handlerGrpcEndpoint := handlerGrpcP.NewEndpoint(usecaseEndpoint)

	// usr
	domainUsrRepoDb := domainUsrRepoDbP.New(a.pgpool)
	domainUsrService := domainUsrServiceP.New(domainUsrRepoDb)
	usecaseUsr := usecaseUsrP.New(domainUsrService, sessionService)
	handlerGrpcUsr := handlerGrpcP.NewUsr(usecaseUsr)

	// snapshot
	snapshotService := serviceSnapshotP.New(a.ctx, domainRootService, domainAppService, domainEndpointService)
	usecaseSnapshot := usecaseSnapshotP.New(snapshotService)
	handlerGrpcSnapshot := handlerGrpcP.NewSnapshot(usecaseSnapshot)

	// stats
	usecaseStats := usecaseStatsP.New(domainRootService, domainAppService, domainEndpointService, domainUsrService)
	handlerGrpcStats := handlerGrpcP.NewStats(usecaseStats)

	// migrate
	serviceMigrate := serviceMigrateP.New(
		config.Conf.LegacyDMBaseURL,
		config.Conf.LegacyDMRefreshToken,
		domainRootService,
		domainAppService,
		domainEndpointService,
	)
	usecaseMigrate := usecaseMigrateP.New(serviceMigrate, sessionService)
	handlerGrpcMigrate := handlerGrpcP.NewMigrate(usecaseMigrate)

	// grpc server
	{
		a.grpcServer = NewGrpcServer("main", sessionService, func(server *grpc.Server) {
			ruto_v1.RegisterRootServer(server, handlerGrpcRoot)
			ruto_v1.RegisterAppServer(server, handlerGrpcApp)
			ruto_v1.RegisterEndpointServer(server, handlerGrpcEndpoint)
			ruto_v1.RegisterSnapshotServer(server, handlerGrpcSnapshot)
			ruto_v1.RegisterStatsServer(server, handlerGrpcStats)
			ruto_v1.RegisterUsrServer(server, handlerGrpcUsr)
			ruto_v1.RegisterMigrateServer(server, handlerGrpcMigrate)
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
				ruto_v1.RegisterAppHandler,
				ruto_v1.RegisterEndpointHandler,
				ruto_v1.RegisterSnapshotHandler,
				ruto_v1.RegisterStatsHandler,
				ruto_v1.RegisterUsrHandler,
				ruto_v1.RegisterMigrateHandler,
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
	if config.Conf.GwPort > 0 {
		a.gw, err = serviceGwP.New(a.ctx, config.Conf.GwPort, config.Conf.SnapshotGrpcAddress)
		errCheck(err, "gw-server New")
	}
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
	if a.gw != nil {
		a.gw.Start()
	}
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
	if a.gw != nil {
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
