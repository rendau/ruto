package grpc

import (
	"sync/atomic"

	gogrpc "google.golang.org/grpc"
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
