package http

import (
	"fmt"
	"net/http"

	"github.com/rendau/ruto/internal/model/config"
	"github.com/rendau/ruto/internal/service/gw/handler/http/middleware"
	"github.com/rendau/ruto/internal/service/gw/handler/http/proxy"
)

type Service struct {
	h http.Handler
}

func New() *Service {
	return &Service{
		h: http.NotFoundHandler(),
	}
}

func (s *Service) Build(conf *config.Root) error {
	err := conf.Normalize()
	if err != nil {
		return fmt.Errorf("config normalize: %w", err)
	}

	s.h, err = buildHandler(conf)
	if err != nil {
		return fmt.Errorf("buildHandler: %w", err)
	}

	return nil
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

	for _, app := range conf.Apps {
		appHandler := proxy.NewProxy(app)

		for _, endpoint := range app.Endpoints {
			mux.Handle(
				createRoute(app, endpoint),
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

func createRoute(app *config.App, endpoint *config.Endpoint) string {
	if endpoint.Path == "" {
		return endpoint.Method + " " + app.PublicPathPrefix
	}
	return endpoint.Method + " " + app.PublicPathPrefix + "/" + endpoint.Path
}
