package log

import (
	"log/slog"
	"time"

	domAppModel "github.com/rendau/ruto/internal/domain/app/model"
	domEndpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
)

type serveFunc func() ([]any, string, error)

type Service struct {
	app       *domAppModel.App
	ep        *domEndpointModel.Endpoint
	method    string
	accessLog bool
}

func New(
	app *domAppModel.App,
	ep *domEndpointModel.Endpoint,
	method string,
	accessLog bool,
) *Service {
	return &Service{
		app:       app,
		ep:        ep,
		method:    method,
		accessLog: accessLog,
	}
}

func (s *Service) Serve(f serveFunc) {
	startAt := time.Now()

	logArgs, status, err := f()

	if !s.accessLog && err == nil {
		return
	}

	logArgs = append(logArgs,
		"app_name", s.app.Name,
		"method", s.method,
		"status", status,
		"duration", time.Since(startAt).String(),
	)

	logMessageSuffix := s.method + " (" + status + ")"

	if err != nil {
		logArgs = append(logArgs, "error", err.Error())
		slog.Info("access log error "+logMessageSuffix, logArgs...)
	} else {
		slog.Info("access log "+logMessageSuffix, logArgs...)
	}
}
