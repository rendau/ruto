package main

import (
	"fmt"
	"log/slog"
	"os"

	appGateway "github.com/rendau/ruto/internal/app/gateway"
)

func main() {
	a := appGateway.New()
	checkError(a.Init(), "app init")
	a.Start()
	a.Wait()
	a.Stop()
	a.Exit()
}

func checkError(err error, msg string) {
	if err == nil {
		return
	}
	if msg != "" {
		err = fmt.Errorf("%s: %w", msg, err)
	}
	slog.Error(err.Error())
	os.Exit(1)
}
