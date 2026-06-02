package grpc

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/mwitkow/grpc-proxy/proxy"
	gogrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	domEndpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
	domRootModel "github.com/rendau/ruto/internal/domain/root/model"
	serviceAuth "github.com/rendau/ruto/internal/service/gw/service/auth"
	serviceLog "github.com/rendau/ruto/internal/service/gw/service/log"
	serviceMetrics "github.com/rendau/ruto/internal/service/gw/service/metrics"
)

type Service struct {
	conns             sync.Map
	routes            map[string]*route
	transparentHandle gogrpc.StreamHandler
}

func New(snapshot *domRootModel.Root, accessLog bool) (*Service, error) {
	service := &Service{}
	service.transparentHandle = proxy.TransparentHandler(service.director)

	routes, err := service.buildRoutes(snapshot, accessLog)
	if err != nil {
		return nil, fmt.Errorf("build routes: %w", err)
	}
	service.routes = routes

	return service, nil
}

func (s *Service) buildRoutes(snapshot *domRootModel.Root, accessLog bool) (map[string]*route, error) {
	routes := make(map[string]*route)

	for _, app := range snapshot.ActiveApps() {
		targetGrpcAddress := app.GrpcAddress()
		if targetGrpcAddress == "" {
			continue
		}

		for _, ep := range app.ActiveEndpoints() {
			if ep.Type != domEndpointModel.TypeGRPC {
				continue
			}

			routeKey := composeRouteKey(app.Name, ep.Grpc.Path)
			if _, exists := routes[routeKey]; exists {
				slog.Warn(
					"duplicate grpc endpoint path for app name",
					"path", ep.Grpc.Path,
					"app", app.Name,
					"endpoint", ep.Id,
				)
				continue
			}

			routeName := fmt.Sprintf("(%s)%s", app.Name, ep.Grpc.Path)

			rt := &route{
				app:               app,
				endpoint:          ep,
				targetGrpcAddress: targetGrpcAddress,
			}

			authService := serviceAuth.New(snapshot, app, ep)
			logService := serviceLog.New(app, ep, "GRPC "+routeName, accessLog)
			metricsService := serviceMetrics.New(app, ep, "GRPC "+routeName)

			rt.handler = chain(s.transparentHandle,
				newMetricsMiddleware(metricsService),
				newRequestLogMiddleware(logService, app, ep),
				newAuthMiddleware(authService),
			)

			routes[routeKey] = rt
		}
	}

	return routes, nil
}

func (s *Service) Handle(_ any, stream gogrpc.ServerStream) error {
	fullMethod, ok := gogrpc.MethodFromServerStream(stream)
	if !ok || strings.TrimSpace(fullMethod) == "" {
		return status.Error(codes.InvalidArgument, "missing method")
	}

	rt := s.resolveRoute(metadataFromContext(stream.Context()), fullMethod)
	if rt == nil {
		return status.Error(codes.NotFound, "route not found")
	}

	streamWithRoute := &serverStreamWithContext{
		ServerStream: stream,
		ctx:          contextWithRoute(stream.Context(), rt),
	}

	err := rt.handler(nil, streamWithRoute)
	if err == nil {
		return nil
	}
	if grpcStatus, ok := status.FromError(err); ok {
		return grpcStatus.Err()
	}

	return status.Errorf(codes.Internal, "gateway forward failed: %v", err)
}

func (s *Service) director(ctx context.Context, _ string) (context.Context, gogrpc.ClientConnInterface, error) {
	rt, ok := routeFromContext(ctx)
	if !ok || rt == nil {
		return nil, nil, status.Error(codes.NotFound, "route not found")
	}

	conn, err := s.getConn(rt.targetGrpcAddress)
	if err != nil {
		return nil, nil, status.Errorf(codes.Unavailable, "dial backend: %v", err)
	}

	return ctx, conn, nil
}

func (s *Service) resolveRoute(md metadata.MD, fullMethod string) *route {
	appName := strings.TrimSpace(metadataFirstValue(md, metadataHeaderAppName))
	if appName == "" {
		return nil
	}

	rt, ok := s.routes[composeRouteKey(appName, fullMethod)]
	if !ok {
		return nil
	}

	return rt
}

func (s *Service) getConn(target string) (*gogrpc.ClientConn, error) {
	if conn, ok := s.conns.Load(target); ok {
		return conn.(*gogrpc.ClientConn), nil
	}

	newConn, err := gogrpc.NewClient(target, gogrpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("gogrpc.NewClient: %w", err)
	}

	actual, loaded := s.conns.LoadOrStore(target, newConn)
	if loaded {
		_ = newConn.Close()
		return actual.(*gogrpc.ClientConn), nil
	}

	return newConn, nil
}

func composeRouteKey(appName, method string) string {
	return strings.ToLower(appName) + routeKeySep + method
}

func remoteAddrFromContext(ctx context.Context) string {
	if p, ok := peer.FromContext(ctx); ok && p != nil && p.Addr != nil {
		return p.Addr.String()
	}
	return ""
}
