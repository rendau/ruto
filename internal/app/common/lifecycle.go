package common

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

func WaitSignal() {
	signalCtx, signalCtxCancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer signalCtxCancel()
	<-signalCtx.Done()
}
