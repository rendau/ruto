package http

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync/atomic"

	"github.com/rendau/ruto/internal/model/config"
	"github.com/rendau/ruto/internal/service/gw/handler/http/middleware"
)

type Service struct {
	handlerStore atomic.Pointer[handlerHolderT]
}

type handlerHolderT struct {
	h http.Handler
}

func New() *Service {
	s := &Service{}
	s.handlerStore.Store(&handlerHolderT{h: http.NotFoundHandler()})
	return s
}

func (s *Service) SetConfig(conf *config.Root) error {
	if err := conf.Validate(); err != nil {
		return fmt.Errorf("config validate: %w", err)
	}

	h, err := buildHandler(conf)
	if err != nil {
		return err
	}

	s.handlerStore.Store(&handlerHolderT{h: h})
	return nil
}

func (s *Service) Validate(conf *config.Root) error {
	if err := conf.Validate(); err != nil {
		return fmt.Errorf("config validate: %w", err)
	}

	if _, err := buildHandler(conf); err != nil {
		return fmt.Errorf("config build handler: %w", err)
	}

	return nil
}

func (s *Service) Handler() http.Handler {
	h := s.handlerStore.Load()
	if h == nil || h.h == nil {
		return http.NotFoundHandler()
	}
	return h.h
}

func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.Handler().ServeHTTP(w, r)
}

func buildHandler(conf *config.Root) (http.Handler, error) {
	mux := http.NewServeMux()

	routeToEndpoint := make(map[string]string)

	for appIdx := range conf.Apps {
		app := conf.Apps[appIdx]

		backendBaseURL, err := parseBackendHost(app.Backend.Host)
		if err != nil {
			return nil, fmt.Errorf("apps[%d].backend.host: %w", appIdx, err)
		}

		for endpointIdx := range app.Endpoints {
			endpoint := app.Endpoints[endpointIdx]

			routePath := joinPath(app.PublicPath, endpoint.Path)
			pattern := endpoint.Method + " " + routePath

			if existingEndpointID, ok := routeToEndpoint[pattern]; ok {
				return nil, fmt.Errorf("duplicate route %q for endpoints %q and %q", pattern, existingEndpointID, endpoint.Id)
			}

			backendPath := joinPath(backendBaseURL.Path, app.Backend.Path, endpoint.Backend.Path)
			endpointHandler := newReverseProxyHandler(backendBaseURL, backendPath, endpoint.Id)
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

func parseBackendHost(rawHost string) (*url.URL, error) {
	parsed, err := url.Parse(rawHost)
	if err != nil {
		return nil, err
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return nil, errors.New("must include scheme and host")
	}
	return parsed, nil
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

func newReverseProxyHandler(targetBaseURL *url.URL, backendPath, endpointID string) http.Handler {
	targetScheme := targetBaseURL.Scheme
	targetHost := targetBaseURL.Host
	targetRawQuery := targetBaseURL.RawQuery

	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = targetScheme
			req.URL.Host = targetHost
			req.URL.Path = backendPath
			req.URL.RawPath = ""
			req.URL.RawQuery = mergeRawQuery(targetRawQuery, req.URL.RawQuery)
			req.Host = targetHost
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			slog.Error("reverse proxy request failed",
				"error", err,
				"endpoint_id", endpointID,
				"method", r.Method,
				"path", r.URL.Path,
			)
			http.Error(w, http.StatusText(http.StatusBadGateway), http.StatusBadGateway)
		},
	}

	return proxy
}

func mergeRawQuery(baseQuery, requestQuery string) string {
	switch {
	case baseQuery == "":
		return requestQuery
	case requestQuery == "":
		return baseQuery
	default:
		return baseQuery + "&" + requestQuery
	}
}
