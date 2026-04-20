package http

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/rendau/ruto/internal/model/config"
	"github.com/rendau/ruto/internal/service/gw/handler/http/middleware"
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

	routeToEndpoint := make(map[string]string)

	for _, app := range conf.Apps {

		for _, endpoint := range app.Endpoints {
			routePath := joinPath(app.PublicPathPrefix, endpoint.Path)
			pattern := endpoint.Method + " " + routePath

			if existingEndpointID, ok := routeToEndpoint[pattern]; ok {
				return nil, fmt.Errorf("duplicate route %q for endpoints %q and %q", pattern, existingEndpointID, endpoint.Id)
			}

			backendPath := joinPath(backendBaseURL.Path, app.Backend.Path, endpoint.Backend.Path)
			endpointHandler := newProxy(app)
			endpointHandler = middleware.Chain(endpointHandler,
				middleware.NewIPValidation(endpoint.IpValidation.AllowedIps),
			)

			mux.Handle(pattern, endpointHandler)
			routeToEndpoint[pattern] = endpoint.Id
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
