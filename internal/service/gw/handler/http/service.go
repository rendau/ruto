package http

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"

	endpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
	rootModel "github.com/rendau/ruto/internal/domain/root/model"
	"github.com/rendau/ruto/internal/service/gw/handler/http/middleware"
	"github.com/rendau/ruto/internal/service/gw/handler/http/proxy"
)

type Service struct {
	h http.Handler
}

func New(snapshot *rootModel.Root, accessLog bool) (*Service, error) {
	handler, err := buildHandler(snapshot, accessLog)
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

func buildHandler(snapshot *rootModel.Root, accessLog bool) (_ http.Handler, finalErr error) {
	defer func() {
		if r := recover(); r != nil {
			finalErr = fmt.Errorf("panic: %v", r)
		}
	}()

	router := chi.NewRouter()
	sharedTransport := proxy.NewTransport()

	for _, app := range snapshot.ActiveApps() {
		endpoints := app.ActiveEndpoints()
		if len(endpoints) == 0 {
			continue
		}

		appProxyHandler := http.StripPrefix(app.PathPrefix, proxy.NewProxy(app, "", sharedTransport))

		for _, ep := range endpoints {
			if ep.Type == "" || ep.Type == endpointModel.TypeHTTP {
				continue
			}

			routePath := app.GetFullPathForEndpoint(ep.Path)

			proxyHandler := appProxyHandler
			if ep.Backend.CustomPath != "" {
				proxyHandler = http.StripPrefix(
					app.PathPrefix,
					proxy.NewProxy(app, ep.Backend.CustomPath, sharedTransport),
				)
			}

			handler := middleware.Chain(proxyHandler,
				middleware.NewMetrics(app, ep, routePath),
				middleware.NewRequestLog(app, ep, routePath, accessLog),
				middleware.NewAuth(snapshot, app, ep),
			)

			if ep.Method == "*" {
				router.Handle(routePath, handler)
			} else {
				router.Method(ep.Method, routePath, handler)
			}
		}
	}

	return middleware.NewCors(snapshot.Cors)(router), nil
}
