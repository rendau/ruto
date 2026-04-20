package http

import (
	"fmt"
	"net/http"
	"strings"

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

func buildHandler(conf *config.Root) (http.Handler, error) {
	mux := http.NewServeMux()

	for _, app := range conf.Apps {
		endpointHandler := proxy.NewProxy(app)

		for _, endpoint := range app.Endpoints {
			routePath := joinPath(app.PublicPathPrefix, endpoint.Path)
			pattern := endpoint.Method + " " + routePath

			mux.Handle(pattern, endpointHandler)
		}
	}

	return middleware.Chain(mux,
		middleware.NewTimeout(conf.Timeout.Global),
		middleware.NewCors(conf.Cors),
	), nil
}

func joinPath(parts ...string) string {
	cleanParts := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.Trim(part, "/")
		if part == "" {
			continue
		}
		cleanParts = append(cleanParts, part)
	}
	if len(cleanParts) == 0 {
		return "/"
	}
	return "/" + strings.Join(cleanParts, "/")
}
