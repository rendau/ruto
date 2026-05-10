package http

import (
	"fmt"
	"net/http"

	"github.com/rendau/ruto/internal/domain/config/model"
	"github.com/rendau/ruto/internal/service/gw/handler/http/middleware"
	"github.com/rendau/ruto/internal/service/gw/handler/http/proxy"
)

type Service struct {
	h http.Handler
}

func New(conf *model.Root) (*Service, error) {
	err := conf.Normalize()
	if err != nil {
		return nil, fmt.Errorf("config normalize: %w", err)
	}

	handler, err := buildHandler(conf)
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

func buildHandler(conf *model.Root) (_ http.Handler, finalErr error) {
	defer func() {
		if r := recover(); r != nil {
			finalErr = fmt.Errorf("panic: %v", r)
		}
	}()

	mux := http.NewServeMux()

	var routePattern string

	for _, app := range conf.Apps {
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
					middleware.NewWithEndpoint(endpoint),
					middleware.NewStripPrefix(app.PathPrefix),
				),
			)
		}
	}

	// return middleware.Chain(mux,
	// 	middleware.NewTimeout(conf.Timeout.Global),
	// 	middleware.NewCors(conf.Cors),
	// ), nil

	return mux, nil
}
