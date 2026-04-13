package http

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"
)

type Service struct {
	handlerWrapper *handlerWrapperT
	server         *http.Server
}

func New(port int) *Service {
	handlerWrapper := newHandlerWrapper()

	server := &http.Server{
		Addr:              ":" + strconv.Itoa(port),
		Handler:           handlerWrapper,
		ReadHeaderTimeout: 4 * time.Second,
	}

	return &Service{
		handlerWrapper: handlerWrapper,
		server:         server,
	}
}

func (s *Service) Run() {
	go s.run()
	slog.Info("http-server started " + s.server.Addr)
}

func (s *Service) run() {
	err := s.server.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("http-server ListenAndServe",
			"error", err,
			"addr", s.server.Addr,
		)
	}
}

func (s *Service) SetHandler(h http.Handler) {
	s.handlerWrapper.setHandler(h)
}

func (s *Service) Stop(timeout time.Duration) error {
	ctx, ctxCancel := context.WithTimeout(context.Background(), timeout)
	defer ctxCancel()

	err := s.server.Shutdown(ctx)
	if err != nil {
		return fmt.Errorf("http-server Shutdown error: %w", err)
	}

	return nil
}
