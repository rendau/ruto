package grpc

import (
	gogrpc "google.golang.org/grpc"
	reflectionv1 "google.golang.org/grpc/reflection/grpc_reflection_v1"
)

type Handler interface {
	Handle(any, gogrpc.ServerStream) error
}

type ReflectionHandler interface {
	ServerReflectionInfo(reflectionv1.ServerReflection_ServerReflectionInfoServer) error
}
