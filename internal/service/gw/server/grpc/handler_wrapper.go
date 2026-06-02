package grpc

import (
	"sync/atomic"

	gogrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	reflectionv1alpha "google.golang.org/grpc/reflection/grpc_reflection_v1alpha"
	"google.golang.org/grpc/status"
)

type handlerWrapperT struct {
	hhStore atomic.Pointer[handlerHolderT]
}

type handlerHolderT struct {
	h Handler
}

func newHandlerWrapper() *handlerWrapperT {
	wrapper := &handlerWrapperT{}
	wrapper.hhStore.Store(&handlerHolderT{
		h: unconfiguredHandler{},
	})
	return wrapper
}

func (s *handlerWrapperT) setHandler(h Handler) {
	if h != nil {
		s.hhStore.Store(&handlerHolderT{h: h})
	}
}

func (s *handlerWrapperT) Handle(srv any, stream gogrpc.ServerStream) error {
	holder := s.hhStore.Load()
	return holder.h.Handle(srv, stream)
}

func (s *handlerWrapperT) ServerReflectionInfo(stream reflectionv1alpha.ServerReflection_ServerReflectionInfoServer) error {
	holder := s.hhStore.Load()
	handler, ok := holder.h.(ReflectionHandler)
	if !ok {
		return status.Error(codes.Unimplemented, "gateway grpc reflection is not configured")
	}
	return handler.ServerReflectionInfo(stream)
}
