package grpc

import (
	"sync/atomic"

	gogrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type handlerWrapper struct {
	handler atomic.Pointer[gogrpc.StreamHandler]
}

func newHandlerWrapper() *handlerWrapper {
	w := &handlerWrapper{}
	defaultHandler := gogrpc.StreamHandler(func(_ any, _ gogrpc.ServerStream) error {
		return status.Error(codes.Unimplemented, "gateway grpc proxy is not configured")
	})
	w.handler.Store(&defaultHandler)
	return w
}

func (w *handlerWrapper) Handle(srv any, stream gogrpc.ServerStream) error {
	h := w.handler.Load()
	return (*h)(srv, stream)
}

func (w *handlerWrapper) setHandler(handler gogrpc.StreamHandler) {
	if handler == nil {
		return
	}
	w.handler.Store(&handler)
}
