package grpc

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/mwitkow/grpc-proxy/proxy"
	gogrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	domEndpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
	domRootModel "github.com/rendau/ruto/internal/domain/root/model"
	serviceAuth "github.com/rendau/ruto/internal/service/gw/service/auth"
	"github.com/rendau/ruto/internal/service/gw/service/blocklist"
	"github.com/rendau/ruto/internal/service/gw/service/clientstat"
	serviceLog "github.com/rendau/ruto/internal/service/gw/service/log"
	serviceMetrics "github.com/rendau/ruto/internal/service/gw/service/metrics"
	"github.com/rendau/ruto/internal/service/gw/service/ratelimit"
)

type Service struct {
	conns             *Conns
	limits            *ratelimit.Service
	blocks            *blocklist.Guard
	stats             *clientstat.Meter
	routes            map[string]*route
	reflectionRoutes  map[string]*reflectionRoute
	transparentHandle gogrpc.StreamHandler
}

// New builds the handler for a snapshot. conns, limits, blocks and stats are
// owned by the caller and outlive the handler, see Conns. A nil limits turns
// rate limiting off, a nil blocks the block list, a nil stats the client stats.
func New(
	snapshot *domRootModel.Root,
	conns *Conns,
	limits *ratelimit.Service,
	blocks *blocklist.Service,
	stats *clientstat.Service,
) (*Service, error) {
	service := &Service{
		conns:  conns,
		limits: limits,
		blocks: blocks.Guard(snapshot, blocklist.ProtocolGRPC),
		stats:  stats.Meter(snapshot),
	}
	service.transparentHandle = proxy.TransparentHandler(service.director)

	routes, err := service.buildRoutes(snapshot)
	if err != nil {
		return nil, fmt.Errorf("build routes: %w", err)
	}
	service.routes = routes
	service.reflectionRoutes = service.buildReflectionRoutes(snapshot)

	return service, nil
}

func (s *Service) buildRoutes(snapshot *domRootModel.Root) (map[string]*route, error) {
	routes := make(map[string]*route)

	for _, app := range snapshot.ActiveApps() {
		targetGrpcAddress := app.Backend.GrpcUrl
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
				backendHeaders:    ep.Backend.Headers,
			}

			authService := serviceAuth.New(ep)
			logService := serviceLog.New(app, ep, "GRPC "+routeName, ep.Logging, snapshot.LogOwnResponseErrors)
			metricsService := serviceMetrics.New(app, ep, "GRPC "+routeName)

			rt.handler = chain(s.transparentHandle,
				newMetricsMiddleware(metricsService),
				newRequestLogMiddleware(logService, app, ep),
				newRateLimitMiddleware(s.limits.Endpoint(snapshot, app, ep, "GRPC "+routeName)),
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

	md := metadataFromContext(stream.Context())
	client, blocked := s.admit(stream.Context(), md)
	if blocked {
		return errBlocked
	}

	rt := s.resolveRoute(md, fullMethod)
	if rt == nil {
		return status.Error(codes.NotFound, "route not found")
	}

	streamWithRoute := &serverStreamWithContext{
		ServerStream: stream,
		ctx:          contextWithRoute(clientstat.WithClient(stream.Context(), client), rt),
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

// admit counts the request for its client and checks the block list. It goes
// before routing, rate limits and auth, for unknown methods too.
func (s *Service) admit(ctx context.Context, md metadata.MD) (*clientstat.Client, bool) {
	remoteAddr := remoteAddrFromContext(ctx)
	header := headersFromMetadata(md)
	client := s.stats.Start(remoteAddr, header)
	if !s.blocks.Active() {
		return client, false
	}
	return client, s.blocks.BlockedClient(client, remoteAddr, header)
}

func (s *Service) director(ctx context.Context, _ string) (context.Context, gogrpc.ClientConnInterface, error) {
	rt, ok := routeFromContext(ctx)
	if !ok || rt == nil {
		return nil, nil, status.Error(codes.NotFound, "route not found")
	}

	conn, err := s.conns.Get(rt.targetGrpcAddress)
	if err != nil {
		return nil, nil, status.Errorf(codes.Unavailable, "dial backend: %v", err)
	}
	if len(rt.backendHeaders) > 0 {
		md, _ := metadata.FromIncomingContext(ctx)
		if md == nil {
			md = metadata.MD{}
		}
		for key, value := range rt.backendHeaders {
			md.Set(key, value)
		}
		ctx = metadata.NewOutgoingContext(ctx, md)
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

// Targets lists the backend addresses the routes of this handler lead to.
func (s *Service) Targets() map[string]struct{} {
	result := make(map[string]struct{}, len(s.routes))
	for _, rt := range s.routes {
		result[rt.targetGrpcAddress] = struct{}{}
	}
	return result
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
