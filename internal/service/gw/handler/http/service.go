package http

import (
	"fmt"
	"net/http"

	rootModel "github.com/rendau/ruto/internal/domain/root/model"
	"github.com/rendau/ruto/internal/service/gw/handler/http/middleware"
	"github.com/rendau/ruto/internal/service/gw/handler/http/proxy"
)

type Service struct {
	h http.Handler
}

func New(snapshot *rootModel.Root) (*Service, error) {
	err := snapshot.Normalize()
	if err != nil {
		return nil, fmt.Errorf("snapshot normalize: %w", err)
	}

	handler, err := buildHandler(snapshot)
	if err != nil {
		return nil, fmt.Errorf("buildHandler: %w", err)
	}

	return &Service{
		h: handler,
	}, nil
}

func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.h.ServeHTTP(w, r)
}

func buildHandler(snapshot *rootModel.Root) (_ http.Handler, finalErr error) {
	defer func() {
		if r := recover(); r != nil {
			finalErr = fmt.Errorf("panic: %v", r)
		}
	}()

	mux := http.NewServeMux()

	var routePattern string

	for _, app := range snapshot.Apps {
		appHandler := proxy.NewProxy(app)

		for _, endpoint := range app.Endpoints {
			if endpoint.Path == "" {
				routePattern = endpoint.Method + " " + app.PathPrefix
			}
			routePattern = endpoint.Method + " " + app.PathPrefix + "/" + endpoint.Path

			mux.Handle(
				routePattern,
				middleware.Chain(
					appHandler,
					middleware.NewWithRequest(snapshot, app, endpoint),
					middleware.NewStripPrefix(app.PathPrefix),
				),
			)
		}
	}

	// return middleware.Chain(mux,
	// 	middleware.NewTimeout(snapshot.Timeout.Global),
	// 	middleware.NewCors(snapshot.Cors),
	// ), nil

	return mux, nil
}
