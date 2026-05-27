package gateway

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/rendau/ruto/internal/app/common"
	configGateway "github.com/rendau/ruto/internal/config/gateway"
	serviceGwP "github.com/rendau/ruto/internal/service/gw"
)

type App struct {
	ctx       context.Context
	ctxCancel context.CancelFunc

	gw *serviceGwP.Service

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
		configGateway.Conf.CoreGrpcAddress,
		configGateway.Conf.LogRequests,
	)
	if err != nil {
		return fmt.Errorf("gw-server New: %w", err)
	}

	a.gw = gwService

	return nil
}

func (a *App) Start() {
	slog.Info("Starting gateway")
	a.gw.Start()
}

func (a *App) Wait() {
	common.WaitSignal()
}

func (a *App) Stop() {
	slog.Info("Shutting down gateway...")

	a.ctxCancel()
	if err := a.gw.Stop(time.Minute); err != nil {
		slog.Error("gw stop error", "error", err)
		a.exitCode = 1
	}
}

func (a *App) Exit() {
	if a.exitCode != 0 {
		slog.Error("gateway finished with errors", "exit_code", a.exitCode)
	}
}
