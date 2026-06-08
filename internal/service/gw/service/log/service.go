package log

import (
	"log/slog"
	"time"

	"github.com/rendau/ruto/internal/constant"
	domAppModel "github.com/rendau/ruto/internal/domain/app/model"
	domEndpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
	loggingModel "github.com/rendau/ruto/internal/domain/logging/model"
)

type serveFunc func() ([]any, string, error)

type Service struct {
	app    *domAppModel.App
	ep     *domEndpointModel.Endpoint
	method string
	level  string
}

func New(
	app *domAppModel.App,
	ep *domEndpointModel.Endpoint,
	method string,
	logging loggingModel.Logging,
) *Service {
	return &Service{
		app:    app,
		ep:     ep,
		method: method,
		level:  logging.EffectiveLevel(),
	}
}

func (s *Service) Serve(f serveFunc) {
	startAt := time.Now()

	logArgs, status, err := f()

	// level "none" logs nothing (not even errors); "error" logs only failed
	// requests; "all" logs everything.
	if s.level == constant.LoggingLevelNone {
		return
	}
	if s.level != constant.LoggingLevelAll && err == nil {
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
