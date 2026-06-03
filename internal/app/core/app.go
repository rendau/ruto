package core

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/rendau/ruto/internal/app/common"
	configCore "github.com/rendau/ruto/internal/config/core"
	domainAppRepoDbP "github.com/rendau/ruto/internal/domain/app/repo/db"
	domainAppServiceP "github.com/rendau/ruto/internal/domain/app/service"
	domainEndpointRepoDbP "github.com/rendau/ruto/internal/domain/endpoint/repo/db"
	domainEndpointServiceP "github.com/rendau/ruto/internal/domain/endpoint/service"
	domainRootRepoDbP "github.com/rendau/ruto/internal/domain/root/repo/db"
	domainRootServiceP "github.com/rendau/ruto/internal/domain/root/service"
	sessionModel "github.com/rendau/ruto/internal/domain/session/model"
	domainSessionServiceP "github.com/rendau/ruto/internal/domain/session/service"
	domainSnapshotRepoDbP "github.com/rendau/ruto/internal/domain/snapshot/repo/db"
	domainSnapshotServiceP "github.com/rendau/ruto/internal/domain/snapshot/service"
	domainUsrRepoDbP "github.com/rendau/ruto/internal/domain/usr/repo/db"
	domainUsrServiceP "github.com/rendau/ruto/internal/domain/usr/service"
	handlerGrpcP "github.com/rendau/ruto/internal/handler/grpc"
	cacheRepoMemP "github.com/rendau/ruto/internal/service/cache/repo/mem"
	cacheRepoRedisP "github.com/rendau/ruto/internal/service/cache/repo/redis"
	cacheServiceP "github.com/rendau/ruto/internal/service/cache/service"
	serviceMigrateP "github.com/rendau/ruto/internal/service/migrate"
	serviceSwaggerP "github.com/rendau/ruto/internal/service/swagger"
	usecaseAppP "github.com/rendau/ruto/internal/usecase/app"
	usecaseEndpointP "github.com/rendau/ruto/internal/usecase/endpoint"
	usecaseGatewayP "github.com/rendau/ruto/internal/usecase/gateway"
	usecaseMigrateP "github.com/rendau/ruto/internal/usecase/migrate"
	usecaseRootP "github.com/rendau/ruto/internal/usecase/root"
	usecaseSnapshotP "github.com/rendau/ruto/internal/usecase/snapshot"
	usecaseStatsP "github.com/rendau/ruto/internal/usecase/stats"
	usecaseUsrP "github.com/rendau/ruto/internal/usecase/usr"
	"github.com/rendau/ruto/pkg/proto/ruto_v1"
)

type App struct {
	ctx       context.Context
	ctxCancel context.CancelFunc

	pgpool     *pgxpool.Pool
	grpcServer *GrpcServer
	httpServer *http.Server

	exitCode int
}

func New() *App {
	return &App{}
}

func (a *App) Init() error {
	a.ctx, a.ctxCancel = context.WithCancel(context.Background())

	common.InitLogger(configCore.Conf.Debug, configCore.Conf.LogLevel)

	// pgpool
	pgpool, err := initPgPool(configCore.Conf.PgDsn)
	if err != nil {
		return fmt.Errorf("pgpool init: %w", err)
	}
	a.pgpool = pgpool

	// migrations
	runMigrations()
	slog.Info("PG-migrations have been successfully applied")

	// session
	sessionService := domainSessionServiceP.New(configCore.Conf.AdminJWTSecret)

	// swagger
	swaggerService := serviceSwaggerP.New(10 * time.Second)

	// cache
	var cacheRepo cacheServiceP.RepoI
	if configCore.Conf.RedisAddr != "" {
		cacheRepo = cacheRepoRedisP.New(configCore.Conf.RedisAddr, configCore.Conf.RedisDB, configCore.Conf.RedisPassword)
		slog.Info("Cache initialized with Redis")
	} else {
		cacheRepo = cacheRepoMemP.New()
		slog.Info("Cache initialized with memory")
	}
	cacheService := cacheServiceP.New(cacheRepo, "ruto:")

	// root
	domainRootRepoDb := domainRootRepoDbP.New(a.pgpool)
	domainRootService := domainRootServiceP.New(domainRootRepoDb)
	usecaseRoot := usecaseRootP.New(domainRootService, sessionService)
	handlerGrpcRoot := handlerGrpcP.NewRoot(usecaseRoot)

	// app
	domainAppRepoDb := domainAppRepoDbP.New(a.pgpool)
	domainAppService := domainAppServiceP.New(domainAppRepoDb)
	domainEndpointRepoDb := domainEndpointRepoDbP.New(a.pgpool)
	domainEndpointService := domainEndpointServiceP.New(domainEndpointRepoDb)
	usecaseApp := usecaseAppP.New(domainAppService, domainEndpointService, swaggerService, sessionService, domainRootService)
	handlerGrpcApp := handlerGrpcP.NewApp(usecaseApp)

	if configCore.Conf.AppSwaggerDiscoveryOnStart {
		systemCtx := sessionService.WithContext(a.ctx, &sessionModel.Session{Id: 1, IsAdmin: true})
		go func() {
			if err := usecaseApp.BackfillSwaggerURLs(systemCtx); err != nil {
				slog.Error("app swagger discovery on start failed", "error", err)
			}
		}()
	}

	// endpoint
	usecaseEndpoint := usecaseEndpointP.New(domainEndpointService, sessionService, domainRootService, domainAppService)
	handlerGrpcEndpoint := handlerGrpcP.NewEndpoint(usecaseEndpoint)

	// usr
	domainUsrRepoDb := domainUsrRepoDbP.New(a.pgpool)
	domainUsrService := domainUsrServiceP.New(domainUsrRepoDb)
	usecaseUsr := usecaseUsrP.New(domainUsrService, sessionService)
	handlerGrpcUsr := handlerGrpcP.NewUsr(usecaseUsr)

	// snapshot
	domainSnapshotRepoDb := domainSnapshotRepoDbP.New(a.pgpool)
	domainSnapshotService := domainSnapshotServiceP.New(domainSnapshotRepoDb)
	usecaseSnapshot := usecaseSnapshotP.New(domainSnapshotService, domainRootService, domainAppService, domainEndpointService)
	handlerGrpcSnapshot := handlerGrpcP.NewSnapshot(usecaseSnapshot)

	// stats
	usecaseStats := usecaseStatsP.New(domainRootService, domainAppService, domainEndpointService, domainUsrService)
	handlerGrpcStats := handlerGrpcP.NewStats(usecaseStats)

	// service-migrate (from legacy DM)
	serviceMigrate := serviceMigrateP.New(
		configCore.Conf.LegacyDMBaseURL,
		configCore.Conf.LegacyDMRefreshToken,
		domainRootService,
		domainAppService,
		domainEndpointService,
	)
	usecaseMigrate := usecaseMigrateP.New(serviceMigrate, sessionService)
	handlerGrpcMigrate := handlerGrpcP.NewMigrate(usecaseMigrate)

	// gateway
	usecaseGateway := usecaseGatewayP.New(sessionService, cacheService.NewChildInstance("gateway:"))
	handlerGrpcGateway := handlerGrpcP.NewGateway(usecaseGateway)

	// grpc-server
	a.grpcServer = NewGrpcServer("core", sessionService, func(server *grpc.Server) {
		ruto_v1.RegisterRootServer(server, handlerGrpcRoot)
		ruto_v1.RegisterAppServer(server, handlerGrpcApp)
		ruto_v1.RegisterEndpointServer(server, handlerGrpcEndpoint)
		ruto_v1.RegisterSnapshotServer(server, handlerGrpcSnapshot)
		ruto_v1.RegisterStatsServer(server, handlerGrpcStats)
		ruto_v1.RegisterUsrServer(server, handlerGrpcUsr)
		ruto_v1.RegisterMigrateServer(server, handlerGrpcMigrate)
		ruto_v1.RegisterGatewayServer(server, handlerGrpcGateway)
	})

	// grpc-gateway
	grpcGwHandler, err := GrpcGatewayCreateHandler(func(mux *runtime.ServeMux) error {
		opts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}
		conn, dialErr := grpc.NewClient("localhost:"+strconv.Itoa(configCore.Conf.GrpcPort), opts...)
		if dialErr != nil {
			return fmt.Errorf("grpc.NewClient: %w", dialErr)
		}

		handlers := []func(context.Context, *runtime.ServeMux, *grpc.ClientConn) error{
			ruto_v1.RegisterRootHandler,
			ruto_v1.RegisterAppHandler,
			ruto_v1.RegisterEndpointHandler,
			ruto_v1.RegisterSnapshotHandler,
			ruto_v1.RegisterStatsHandler,
			ruto_v1.RegisterUsrHandler,
			ruto_v1.RegisterMigrateHandler,
			ruto_v1.RegisterGatewayHandler,
		}
		for _, registerHandler := range handlers {
			if registerErr := registerHandler(context.Background(), mux, conn); registerErr != nil {
				return fmt.Errorf("grpc-gateway: register grpc-handler: %w", registerErr)
			}
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("grpc gateway create handler: %w", err)
	}

	handler := http.NewServeMux()
	handler.Handle("/api", http.RedirectHandler("/api/", http.StatusMovedPermanently))
	handler.Handle("/api/", http.StripPrefix("/api", grpcGwHandler))
	handler.Handle("/", NewAdminSPAHandler())

	a.httpServer = &http.Server{
		Addr:              ":" + strconv.Itoa(configCore.Conf.HttpPort),
		Handler:           handler,
		ReadHeaderTimeout: 2 * time.Second,
		ReadTimeout:       time.Minute,
		MaxHeaderBytes:    300 * 1024,
	}

	return nil
}

func (a *App) Start() error {
	slog.Info("Starting core")

	// grpc-server
	if err := a.grpcServer.Start(); err != nil {
		return fmt.Errorf("grpcServer.Start: %w", err)
	}

	// http-server
	go func() {
		err := a.httpServer.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("http server stopped unexpectedly", "error", err)
		}
	}()
	slog.Info("http-server started " + a.httpServer.Addr)

	return nil
}

func (a *App) Wait() {
	common.WaitSignal()
}

func (a *App) Stop() {
	slog.Info("Shutting down core...")

	a.ctxCancel()

	// grpc gateway server
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

func (a *App) Exit() {
	if a.pgpool != nil {
		a.pgpool.Close()
	}

	if a.exitCode != 0 {
		slog.Error("core finished with errors", "exit_code", a.exitCode)
	}
}
