package http

import (
	"fmt"
	"net/http"

	"github.com/samber/lo"

	endpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
	rootModel "github.com/rendau/ruto/internal/domain/root/model"
	"github.com/rendau/ruto/internal/service/gw/handler/http/middleware"
	"github.com/rendau/ruto/internal/service/gw/handler/http/middleware/auth"
	"github.com/rendau/ruto/internal/service/gw/handler/http/proxy"
	"github.com/rendau/ruto/internal/service/gw/jwk"
)

type Service struct {
	h http.Handler
}

func New(snapshot *rootModel.Root, jwkService *jwk.Service) (*Service, error) {
	err := snapshot.Normalize()
	if err != nil {
		return nil, fmt.Errorf("snapshot normalize: %w", err)
	}

	handler, err := buildHandler(snapshot, jwkService)
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

func buildHandler(snapshot *rootModel.Root, jwkService *jwk.Service) (_ http.Handler, finalErr error) {
	defer func() {
		if r := recover(); r != nil {
			finalErr = fmt.Errorf("panic: %v", r)
		}
	}()

	mux := http.NewServeMux()

	var routePattern string

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
				routePattern = endpoint.Method + " " + app.PathPrefix
			} else {
				routePattern = endpoint.Method + " " + app.PathPrefix + "/" + endpoint.Path
			}

			mux.Handle(
				routePattern,
				middleware.Chain(appHandler,
					middleware.NewWithRequest(snapshot, app, endpoint, jwkService),
					auth.New(endpoint),
					middleware.NewStripPrefix(app.PathPrefix),
				),
			)
		}
	}

	return mux, nil
}
