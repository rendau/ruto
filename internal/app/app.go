package app

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/samber/lo"

	"github.com/rendau/ruto/internal/config"
	serviceGwServerHttpP "github.com/rendau/ruto/internal/service/gw_server/http"
)

type App struct {
	pgpool *pgxpool.Pool

	serviceGwServerHttp *serviceGwServerHttpP.Service

	ctx       context.Context
	ctxCancel context.CancelFunc

	exitCode int
}

func (a *App) Init() {
	var err error

	a.ctx, a.ctxCancel = context.WithCancel(context.Background())

	// logger
	initLogger(config.Conf.Debug, config.Conf.LogLevel)

	if config.Conf.Debug {
		slog.Info("DEBUG mode enabled")
	}

	// pgpool
	a.pgpool, err = initPgPool(config.Conf.PgDsn)
	errCheck(err, "pgpool init")

	// gw-server-http
	a.serviceGwServerHttp = serviceGwServerHttpP.New(config.Conf.HttpPort)
}

func (a *App) PreStartHook() {
	// slog.Info("PreStartHook")
}

func (a *App) Start() {
	slog.Info("Starting")

	// gw-server-http
	a.serviceGwServerHttp.Run()
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

	// // gw-server-http
	{
		err := a.serviceGwServerHttp.Stop(15 * time.Second)
		if err != nil {
			slog.Error("http-server Stop error", "error", err)
			a.exitCode = 1
		}
	}
}

func (a *App) WaitJobs() {
	slog.Info("waiting jobs")
}

func (a *App) Exit() {
	slog.Info("Exit")

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
