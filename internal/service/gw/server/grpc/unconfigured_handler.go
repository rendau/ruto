package grpc

import (
	gogrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type unconfiguredHandler struct{}

func (unconfiguredHandler) Handle(any, gogrpc.ServerStream) error {
	return status.Error(codes.Unimplemented, "gateway grpc proxy is not configured")
}
