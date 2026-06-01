package grpc

import (
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"time"

	gogrpc "google.golang.org/grpc"
)

type Service struct {
	addr           string
	server         *gogrpc.Server
	lis            net.Listener
	handlerWrapper *handlerWrapper
}

func New(port int) *Service {
	handlerWrapper := newHandlerWrapper()
	server := gogrpc.NewServer(
		gogrpc.UnknownServiceHandler(handlerWrapper.Handle),
		gogrpc.ForceServerCodec(rawCodec{}),
	)

	return &Service{
		addr:           ":" + strconv.Itoa(port),
		server:         server,
		handlerWrapper: handlerWrapper,
	}
}

func (s *Service) Start() error {
	lis, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("net.Listen: %w", err)
	}
	s.lis = lis

	go func() {
		if serveErr := s.server.Serve(lis); serveErr != nil {
			slog.Error("gw-grpc-server stopped", "error", serveErr)
		}
	}()

	slog.Info("gw-grpc-server started " + lis.Addr().String())
	return nil
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

func (s *Service) SetUnknownHandler(handler gogrpc.StreamHandler) {
	s.handlerWrapper.setHandler(handler)
}
