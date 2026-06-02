package grpc

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"time"

	gogrpc "google.golang.org/grpc"
	reflectionv1alpha "google.golang.org/grpc/reflection/grpc_reflection_v1"
)

type Service struct {
	addr           string
	server         *gogrpc.Server
	lis            net.Listener
	handlerWrapper *handlerWrapperT
}

func New(port int) *Service {
	handlerWr := newHandlerWrapper()
	server := gogrpc.NewServer(
		gogrpc.UnknownServiceHandler(handlerWr.Handle),
	)
	reflectionv1alpha.RegisterServerReflectionServer(server, handlerWr)

	return &Service{
		addr:           ":" + strconv.Itoa(port),
		server:         server,
		handlerWrapper: handlerWr,
	}
}

func (s *Service) Start() {
	go s.run()
	slog.Info("gw-grpc-server started " + s.addr)
}

func (s *Service) run() {
	lis, err := net.Listen("tcp", s.addr)
	if err != nil {
		slog.Error("gw-grpc-server net.Listen",
			"error", fmt.Errorf("net.Listen: %w", err),
			"addr", s.addr,
		)
		return
	}
	s.lis = lis

	serveErr := s.server.Serve(lis)
	if serveErr != nil && !errors.Is(serveErr, gogrpc.ErrServerStopped) {
		slog.Error("gw-grpc-server Serve",
			"error", serveErr,
			"addr", s.addr,
		)
	}
}

func (s *Service) Stop(timeout time.Duration) error {
	done := make(chan struct{})
	go func() {
		s.server.GracefulStop()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-time.After(timeout):
		s.server.Stop()
		return nil
	}
}

func (s *Service) SetHandler(handler Handler) {
	s.handlerWrapper.setHandler(handler)
}
