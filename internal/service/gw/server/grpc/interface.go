package grpc

import (
	gogrpc "google.golang.org/grpc"
)

type Handler interface {
	Handle(any, gogrpc.ServerStream) error
}
