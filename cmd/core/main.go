package main

import (
	"fmt"
	"log/slog"
	"os"

	appCore "github.com/rendau/ruto/internal/app/core"
)

func main() {
	a := appCore.New()
	checkError(a.Init(), "app init")
	checkError(a.Start(), "app start")
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
