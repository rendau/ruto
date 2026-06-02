package grpc

import (
	gogrpc "google.golang.org/grpc"
	reflectionv1alpha "google.golang.org/grpc/reflection/grpc_reflection_v1alpha"
)

type Handler interface {
	Handle(any, gogrpc.ServerStream) error
}

type ReflectionHandler interface {
	ServerReflectionInfo(reflectionv1alpha.ServerReflection_ServerReflectionInfoServer) error
}
