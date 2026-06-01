package grpc

import (
	"context"

	gogrpc "google.golang.org/grpc"
)

type serverStreamWithContext struct {
	gogrpc.ServerStream
	ctx context.Context
}

func (s *serverStreamWithContext) Context() context.Context {
	return s.ctx
}
