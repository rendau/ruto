package grpc

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"

	"github.com/mwitkow/grpc-proxy/proxy"
	gogrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	appModel "github.com/rendau/ruto/internal/domain/app/model"
	endpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
	rootModel "github.com/rendau/ruto/internal/domain/root/model"
	serviceAuth "github.com/rendau/ruto/internal/service/gw/service/auth"
	authModel "github.com/rendau/ruto/internal/service/gw/service/auth/model"
	serviceLog "github.com/rendau/ruto/internal/service/gw/service/log"
	serviceMetrics "github.com/rendau/ruto/internal/service/gw/service/metrics"
)

type Service struct {
	snapshot          *rootModel.Root
	conns             sync.Map
	routes            map[string]*route
	transparentHandle gogrpc.StreamHandler
}

type route struct {
	app      *appModel.App
	endpoint *endpointModel.Endpoint
	auth     *serviceAuth.Service
	log      *serviceLog.Service
	metrics  *serviceMetrics.Service
}

type routeCtxKey struct{}

const (
	metadataHeaderAppName = "x-ruto-app-name"
	routeKeySep           = "\x1f"
)

func New(snapshot *rootModel.Root, accessLog bool) (*Service, error) {
	service := &Service{
		snapshot: snapshot,
	}
	service.transparentHandle = proxy.TransparentHandler(service.director)

	routes, err := service.buildRoutes(accessLog)
	if err != nil {
		return nil, fmt.Errorf("build routes: %w", err)
	}
	service.routes = routes

	return service, nil
}

func (s *Service) buildRoutes(accessLog bool) (map[string]*route, error) {
	routes := make(map[string]*route)

	for _, app := range s.snapshot.ActiveApps() {
		if app.GrpcPort <= 0 {
			continue
		}

		for _, ep := range app.ActiveEndpoints() {
			if ep.Type != endpointModel.TypeGRPC {
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

			routes[routeKey] = &route{
				app:      app,
				endpoint: ep,
				auth:     serviceAuth.New(s.snapshot, app, ep),
				log:      serviceLog.New(app, ep, "GRPC "+routeName, accessLog),
				metrics:  serviceMetrics.New(app, ep, "GRPC "+routeName),
			}
		}
	}

	return routes, nil
}

func (s *Service) Handle(_ any, stream gogrpc.ServerStream) error {
	fullMethod, ok := gogrpc.MethodFromServerStream(stream)
	if !ok || strings.TrimSpace(fullMethod) == "" {
		return status.Error(codes.InvalidArgument, "missing method")
	}

	rt := s.resolveRoute(stream.Context(), fullMethod)
	if rt == nil {
		return status.Error(codes.NotFound, "route not found")
	}

	if rt.auth != nil {
		authReq := authModel.NewAuthRequest()
		authReq.SetHttpHeader(headersFromMetadata(stream.Context()))
		authReq.SetRemoteAddr(remoteAddrFromContext(stream.Context()))
		if !rt.auth.Check(authReq) {
			return status.Error(codes.Unauthenticated, "unauthorized")
		}
	}

	streamWithRoute := &serverStreamWithContext{
		ServerStream: stream,
		ctx:          context.WithValue(stream.Context(), routeCtxKey{}, rt),
	}

	serve := func() error {
		return s.transparentHandle(nil, streamWithRoute)
	}

	var (
		serveErr    error
		statusLabel = codes.OK.String()
	)

	runWithLog := func() {
		if rt.log == nil {
			serveErr, statusLabel = runForwardWithStatus(serve)
			return
		}
		rt.log.Serve(func() ([]any, string, error) {
			serveErr, statusLabel = runForwardWithStatus(serve)
			return []any{
				"app_name", rt.app.Name,
				"grpc_service", rt.endpoint.Grpc.Service,
				"grpc_method", rt.endpoint.Grpc.Method,
				"grpc_path", rt.endpoint.Grpc.Path,
			}, statusLabel, serveErr
		})
	}

	if rt.metrics != nil {
		rt.metrics.Serve(func() string {
			runWithLog()
			return statusLabel
		})
	} else {
		runWithLog()
	}

	if serveErr == nil {
		return nil
	}
	if grpcStatus, ok := status.FromError(serveErr); ok {
		return grpcStatus.Err()
	}

	return status.Errorf(codes.Internal, "gateway forward failed: %v", serveErr)
}

func (s *Service) director(ctx context.Context, fullMethod string) (context.Context, gogrpc.ClientConnInterface, error) {
	rt, ok := ctx.Value(routeCtxKey{}).(*route)
	if !ok || rt == nil {
		rt = s.resolveRoute(ctx, fullMethod)
		if rt == nil {
			return nil, nil, status.Error(codes.NotFound, "route not found")
		}
	}

	target := rt.app.GrpcAddress()
	if target == "" {
		return nil, nil, status.Error(codes.FailedPrecondition, "app grpc_port is not configured")
	}

	conn, err := s.getConn(target)
	if err != nil {
		return nil, nil, status.Errorf(codes.Unavailable, "dial backend: %v", err)
	}

	inMd, _ := metadata.FromIncomingContext(ctx)
	outMd := metadata.MD{}
	for k, values := range inMd {
		if strings.HasPrefix(k, ":") {
			continue
		}
		outMd[k] = append([]string(nil), values...)
	}

	return metadata.NewOutgoingContext(ctx, outMd), conn, nil
}

func (s *Service) resolveRoute(ctx context.Context, fullMethod string) *route {
	appName := strings.TrimSpace(metadataValue(ctx, metadataHeaderAppName))
	if appName == "" {
		return nil
	}

	rt, ok := s.routes[composeRouteKey(appName, fullMethod)]
	if !ok {
		return nil
	}

	return rt
}

func runForwardWithStatus(f func() error) (error, string) {
	err := f()
	if err == nil {
		return nil, codes.OK.String()
	}
	if st, ok := status.FromError(err); ok {
		return err, st.Code().String()
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return err, codes.DeadlineExceeded.String()
	}
	return err, codes.Internal.String()
}

func (s *Service) getConn(target string) (*gogrpc.ClientConn, error) {
	if conn, ok := s.conns.Load(target); ok {
		if existing, ok := conn.(*gogrpc.ClientConn); ok {
			return existing, nil
		}
	}

	newConn, err := gogrpc.NewClient(target, gogrpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	actual, loaded := s.conns.LoadOrStore(target, newConn)
	if loaded {
		_ = newConn.Close()
		return actual.(*gogrpc.ClientConn), nil
	}

	return newConn, nil
}

func headersFromMetadata(ctx context.Context) http.Header {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return make(http.Header)
	}
	result := make(http.Header, len(md))
	for key, values := range md {
		for _, value := range values {
			result.Add(key, value)
		}
	}
	return result
}

func metadataValue(ctx context.Context, key string) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok || len(md) == 0 {
		return ""
	}
	values := md.Get(key)
	if len(values) == 0 {
		return ""
	}
	return values[0]
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
