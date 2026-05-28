package http

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/samber/lo"

	endpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
	rootModel "github.com/rendau/ruto/internal/domain/root/model"
	"github.com/rendau/ruto/internal/service/gw/handler/http/middleware"
	"github.com/rendau/ruto/internal/service/gw/handler/http/middleware/auth"
	"github.com/rendau/ruto/internal/service/gw/handler/http/middleware/auth/jwt"
	"github.com/rendau/ruto/internal/service/gw/handler/http/proxy"
)

type Service struct {
	h http.Handler
}

func New(snapshot *rootModel.Root, jwkGetter jwt.JwkGetterI, logRequests bool) (*Service, error) {
	err := snapshot.Normalize()
	if err != nil {
		return nil, fmt.Errorf("snapshot normalize: %w", err)
	}

	handler, err := buildHandler(snapshot, jwkGetter, logRequests)
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

func buildHandler(snapshot *rootModel.Root, jwkGetter jwt.JwkGetterI, logRequests bool) (_ http.Handler, finalErr error) {
	defer func() {
		if r := recover(); r != nil {
			finalErr = fmt.Errorf("panic: %v", r)
		}
	}()

	router := chi.NewRouter()

	metricsMiddleware := middleware.NewMetrics()

	var routePath string

	for _, app := range snapshot.Apps {
		if !app.Active {
			continue
		}

		// filter endpoints
		filteredEndpoints := lo.Filter(app.Endpoints, func(endpoint *endpointModel.Endpoint, _ int) bool {
			return endpoint.Active
		})
		if len(filteredEndpoints) == 0 {
			continue
		}

		// proxy
		appHandler := proxy.NewProxy(app)

		for _, endpoint := range filteredEndpoints {
			if endpoint.Path == "" {
				routePath = app.PathPrefix
			} else {
				routePath = app.PathPrefix + "/" + endpoint.Path
			}

			handler := middleware.Chain(appHandler,
				middleware.NewWithRequest(snapshot, app, endpoint),
				metricsMiddleware,
				auth.New(snapshot, app, endpoint, jwkGetter),
				middleware.NewStripPrefix(app.PathPrefix),
			)

			if endpoint.Method == "*" {
				router.Handle(routePath, handler)
			} else {
				router.Method(endpoint.Method, routePath, handler)
			}
		}
	}

	outerMiddlewares := []middleware.Middleware{
		middleware.NewRequestLog(logRequests),
	}
	if snapshot.Cors.Enabled {
		outerMiddlewares = append(outerMiddlewares, middleware.NewCors(snapshot.Cors))
	}

	return middleware.Chain(router, outerMiddlewares...), nil
}
