package http

import (
	"fmt"
	"net/http"
	"slices"

	"github.com/go-chi/chi/v5"

	endpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
	rootModel "github.com/rendau/ruto/internal/domain/root/model"
	"github.com/rendau/ruto/internal/service/gw/handler/http/middleware"
	"github.com/rendau/ruto/internal/service/gw/handler/http/proxy"
	"github.com/rendau/ruto/internal/service/gw/service/cache"
)

type Service struct {
	h http.Handler
}

// New builds the handler for a snapshot. transport is owned by the caller and
// outlives the handler: a transport per snapshot would leave the idle
// connections of the previous one open after every config change. The same
// holds for store: cached responses survive a config change.
func New(snapshot *rootModel.Root, transport http.RoundTripper, store *cache.Service) (*Service, error) {
	handler, err := buildHandler(snapshot, transport, store)
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

func buildHandler(snapshot *rootModel.Root, transport http.RoundTripper, store *cache.Service) (_ http.Handler, finalErr error) {
	defer func() {
		if r := recover(); r != nil {
			finalErr = fmt.Errorf("panic: %v", r)
		}
	}()

	router := chi.NewRouter()

	for _, app := range snapshot.ActiveApps() {
		endpoints := app.ActiveEndpoints()
		if len(endpoints) == 0 {
			continue
		}
		// chi stores a "*" handler under every method of its route, overwriting
		// the method handlers registered before it on the same path: registering
		// "*" first lets a concrete method win whatever the snapshot order.
		slices.SortStableFunc(endpoints, func(a, b *endpointModel.Endpoint) int {
			return anyMethodOrder(a) - anyMethodOrder(b)
		})

		appProxyHandler := http.StripPrefix(app.PathPrefix, proxy.NewProxy(app, "", transport))

		for _, ep := range endpoints {
			if ep.Type != "" && ep.Type != endpointModel.TypeHTTP {
				continue
			}

			// route path
			routePath := app.PathPrefix
			if ep.Http.Path != "" {
				routePath += "/" + ep.Http.Path
			}

			proxyHandler := appProxyHandler
			if ep.Backend.CustomPath != "" {
				proxyHandler = http.StripPrefix(
					app.PathPrefix,
					proxy.NewProxy(app, ep.Backend.CustomPath, transport),
				)
			}

			handler := middleware.Chain(proxyHandler,
				middleware.NewMetrics(app, ep, routePath),
				middleware.NewSeenPaths(app, ep),
				middleware.NewRequestLog(app, ep, routePath, snapshot.LogOwnResponseErrors),
				middleware.NewAuth(ep),
				middleware.NewBrowserCache(ep),
				middleware.NewServerCache(app, ep, store),
				middleware.NewBackendRequestParams(ep),
				middleware.NewRequestTransform(ep, snapshot.Transform.MaxWorkers),
				middleware.NewResponseTransform(ep, snapshot.Transform.MaxWorkers),
			)

			routePaths := make([]string, 1, 2)
			routePaths[0] = routePath
			// The same handler, not a copy: one endpoint for metrics and logs,
			// and the backend gets the path as the client sent it.
			if ep.AllowTrailingSlash {
				routePaths = append(routePaths, routePath+"/")
			}
			for _, p := range routePaths {
				if ep.Http.Method == "*" {
					router.Handle(p, handler)
				} else {
					router.Method(ep.Http.Method, p, handler)
				}
			}
		}
	}

	return middleware.NewCors(snapshot.Cors)(router), nil
}

func anyMethodOrder(ep *endpointModel.Endpoint) int {
	if ep.Http.Method == "*" {
		return 0
	}
	return 1
}
