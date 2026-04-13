package http

import (
	"net/http"
	"sync/atomic"
)

type handlerWrapperT struct {
	hhStore atomic.Pointer[handlerHolderT]
}

type handlerHolderT struct {
	h http.Handler
}

func newHandlerWrapper() *handlerWrapperT {
	wrapper := &handlerWrapperT{}
	wrapper.hhStore.Store(&handlerHolderT{
		h: http.NotFoundHandler(),
	})
	return wrapper
}

func (s *handlerWrapperT) setHandler(h http.Handler) {
	if h != nil {
		s.hhStore.Store(&handlerHolderT{h: h})
	}
}

func (s *handlerWrapperT) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	holder := s.hhStore.Load()
	holder.h.ServeHTTP(w, r)
}
