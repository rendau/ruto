package http

import (
	"fmt"
	"net/http"

	"github.com/rendau/ruto/internal/service/gw/handler/http/middleware"
	"github.com/rendau/ruto/internal/service/gw/handler/http/proxy"
	"github.com/rendau/ruto/internal/service/gw/model/config"
)

type Service struct {
	h http.Handler
}

func New(conf *config.Root) (*Service, error) {
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

func buildHandler(conf *config.Root) (_ http.Handler, finalErr error) {
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
				routePattern = endpoint.Method + " " + app.PublicPathPrefix
			}
			routePattern = endpoint.Method + " " + app.PublicPathPrefix + "/" + endpoint.Path

			mux.Handle(
				routePattern,
				middleware.Chain(
					appHandler,
					middleware.NewWithEndpoint(endpoint),
					middleware.NewStripPrefix(app.PublicPathPrefix),
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
