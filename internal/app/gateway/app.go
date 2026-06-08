package gateway

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/rendau/ruto/internal/app/common"
	configGateway "github.com/rendau/ruto/internal/config/gateway"
	"github.com/rendau/ruto/internal/infra/metrics"
	serviceGwP "github.com/rendau/ruto/internal/service/gw"
	serviceGwServiceJwkP "github.com/rendau/ruto/internal/service/gw/service/jwk"
)

type App struct {
	ctx       context.Context
	ctxCancel context.CancelFunc

	gw           *serviceGwP.Service
	systemServer *http.Server

	exitCode int
}

func New() *App {
	return &App{}
}

func (a *App) Init() error {
	a.ctx, a.ctxCancel = context.WithCancel(context.Background())

	common.InitLogger(configGateway.Conf.Debug, configGateway.Conf.LogLevel)

	if configGateway.Conf.HttpPort <= 0 {
		return fmt.Errorf("GW_PORT is required")
	}

	if configGateway.Conf.CoreGrpcAddress == "" {
		return fmt.Errorf("CORE_GRPC_ADDRESS is required")
	}

	gwService, err := serviceGwP.New(
		a.ctx,
		configGateway.Conf.HttpPort,
		configGateway.Conf.GrpcPort,
		configGateway.Conf.CoreGrpcAddress,
	)
	if err != nil {
		return fmt.Errorf("gw-server New: %w", err)
	}

	a.gw = gwService

	// system server
	systemHandler := http.NewServeMux()
	systemHandler.HandleFunc("/healthcheck", func(w http.ResponseWriter, _ *http.Request) {
		if !a.gw.IsReady() {
			http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	})
	if metrics.Enabled {
		systemHandler.Handle("/metrics", promhttp.HandlerFor(metrics.Registry, promhttp.HandlerOpts{}))
	}
	a.systemServer = &http.Server{
		Addr:              ":" + strconv.Itoa(configGateway.Conf.SystemPort),
		Handler:           systemHandler,
		ReadHeaderTimeout: 2 * time.Second,
	}

	return nil
}

func (a *App) Start() {
	slog.Info("Starting gateway")

	a.gw.Start()

	// system server
	go func() {
		err := a.systemServer.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("system server stopped unexpectedly", "error", err)
		}
	}()
	slog.Info("system-server started " + a.systemServer.Addr)
}

func (a *App) Wait() {
	common.WaitSignal()
}

func (a *App) Stop() {
	slog.Info("Shutting down gateway...")

	a.ctxCancel()

	serviceGwServiceJwkP.Stop()

	if err := a.gw.Stop(time.Minute); err != nil {
		slog.Error("gw stop error", "error", err)
		a.exitCode = 1
	}

	// system server
	{
		ctx, ctxCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer ctxCancel()
		if err := a.systemServer.Shutdown(ctx); err != nil {
			slog.Error("system-server shutdown error", "error", err)
			a.exitCode = 1
		}
	}
}

func (a *App) Exit() {
	if a.exitCode != 0 {
		slog.Error("gateway finished with errors", "exit_code", a.exitCode)
	}
}
