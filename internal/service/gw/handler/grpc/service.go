package grpc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"

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
	snapshot *rootModel.Root
	conns    sync.Map
	routes   map[string]map[string]*route
}

type route struct {
	app      *appModel.App
	endpoint *endpointModel.Endpoint
	auth     *serviceAuth.Service
	log      *serviceLog.Service
	metrics  *serviceMetrics.Service
}

type grpcTransport struct {
	stream     gogrpc.ServerStream
	fullMethod string
}

const (
	metadataHeaderAppName = "x-ruto-app-name"
)

func New(snapshot *rootModel.Root, accessLog bool) (*Service, error) {
	if err := snapshot.Normalize(); err != nil {
		return nil, fmt.Errorf("snapshot normalize: %w", err)
	}

	service := &Service{
		snapshot: snapshot,
		routes:   map[string]map[string]*route{},
	}

	if err := service.buildRoutes(accessLog); err != nil {
		return nil, fmt.Errorf("build routes: %w", err)
	}

	return service, nil
}

func (s *Service) Handle(_ any, stream gogrpc.ServerStream) error {
	fullMethod, ok := gogrpc.MethodFromServerStream(stream)
	if !ok || strings.TrimSpace(fullMethod) == "" {
		return status.Error(codes.InvalidArgument, "missing method")
	}

	appName := strings.TrimSpace(metadataValue(stream.Context(), metadataHeaderAppName))
	if appName == "" {
		return status.Errorf(codes.InvalidArgument, "missing metadata header: %s", metadataHeaderAppName)
	}

	routesByMethod, ok := s.routes[fullMethod]
	if !ok {
		return status.Error(codes.NotFound, "endpoint not found")
	}

	rt, ok := routesByMethod[normalizeAppName(appName)]
	if !ok {
		return status.Error(codes.NotFound, "endpoint not found")
	}

	authReq := authModel.NewAuthRequest()
	authReq.SetHttpHeader(headersFromMetadata(stream.Context()))
	authReq.SetRemoteAddr(extractRemoteAddr(stream.Context()))
	if rt.auth != nil && !rt.auth.Check(authReq) {
		return status.Error(codes.Unauthenticated, "unauthorized")
	}

	transport := &grpcTransport{
		stream:     stream,
		fullMethod: fullMethod,
	}
	serve := func() error {
		return s.forward(transport.stream, transport.fullMethod, rt.app)
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
				"host", metadataValue(stream.Context(), ":authority"),
				"remote_addr", authReq.RemoteAddr,
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

func (s *Service) buildRoutes(accessLog bool) error {
	for _, app := range s.snapshot.ActiveApps() {
		if app.GrpcPort <= 0 {
			continue
		}

		for _, ep := range app.ActiveEndpoints() {
			if ep.Type != endpointModel.TypeGRPC {
				continue
			}

			routePath := strings.TrimSpace(ep.Grpc.Path)
			if routePath == "" {
				continue
			}

			appName := strings.TrimSpace(app.Name)
			if appName == "" {
				slog.Warn(
					"skip grpc endpoint with empty app name",
					"path", routePath,
					"app", app.Name,
					"endpoint", ep.Id,
				)
				continue
			}

			routesByMethod := s.routes[routePath]
			if routesByMethod == nil {
				routesByMethod = map[string]*route{}
				s.routes[routePath] = routesByMethod
			}
			appRouteKey := normalizeAppName(appName)
			if _, exists := routesByMethod[appRouteKey]; exists {
				slog.Warn(
					"duplicate grpc endpoint path for app name",
					"path", routePath,
					"app_name", appName,
					"app", app.Name,
					"endpoint", ep.Id,
				)
			}

			routesByMethod[appRouteKey] = &route{
				app:      app,
				endpoint: ep,
				auth:     serviceAuth.New(s.snapshot, app, ep),
				log:      serviceLog.New(app, ep, "GRPC "+routePath, accessLog),
				metrics:  serviceMetrics.New(app, ep, "GRPC "+routePath),
			}
		}
	}
	return nil
}

func normalizeAppName(v string) string {
	return strings.ToLower(strings.TrimSpace(v))
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

func (s *Service) forward(stream gogrpc.ServerStream, fullMethod string, app *appModel.App) error {
	target := app.GrpcAddress()
	if target == "" {
		return status.Error(codes.FailedPrecondition, "app grpc_port is not configured")
	}

	conn, err := s.getConn(target)
	if err != nil {
		return status.Errorf(codes.Unavailable, "dial backend: %v", err)
	}

	inMd, _ := metadata.FromIncomingContext(stream.Context())
	outMd := metadata.MD{}
	for k, values := range inMd {
		if strings.HasPrefix(k, ":") {
			continue
		}
		outMd[k] = append([]string(nil), values...)
	}
	outCtx := metadata.NewOutgoingContext(stream.Context(), outMd)
	clientStream, err := conn.NewStream(
		outCtx,
		&gogrpc.StreamDesc{ServerStreams: true, ClientStreams: true},
		fullMethod,
		gogrpc.ForceCodec(rawCodec{}),
	)
	if err != nil {
		return status.Errorf(codes.Unavailable, "open backend stream: %v", err)
	}

	backendClosed := make(chan error, 1)
	clientClosed := make(chan error, 1)

	go func() {
		clientClosed <- pumpClientToBackend(stream, clientStream)
	}()
	go func() {
		backendClosed <- pumpBackendToClient(stream, clientStream)
	}()

	var firstErr error
	for i := 0; i < 2; i++ {
		select {
		case err := <-clientClosed:
			if err != nil && firstErr == nil {
				firstErr = err
			}
		case err := <-backendClosed:
			if err != nil && firstErr == nil {
				firstErr = err
			}
		}
	}

	stream.SetTrailer(clientStream.Trailer())

	if firstErr != nil {
		return firstErr
	}

	return nil
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

func pumpClientToBackend(serverStream gogrpc.ServerStream, clientStream gogrpc.ClientStream) error {
	for {
		msg := &rawFrame{}
		if err := serverStream.RecvMsg(msg); err != nil {
			if err == io.EOF {
				return clientStream.CloseSend()
			}
			return err
		}
		if err := clientStream.SendMsg(msg); err != nil {
			return err
		}
	}
}

func pumpBackendToClient(serverStream gogrpc.ServerStream, clientStream gogrpc.ClientStream) error {
	headers, err := clientStream.Header()
	if err == nil && len(headers) > 0 {
		if sendErr := serverStream.SendHeader(headers); sendErr != nil {
			return sendErr
		}
	}

	for {
		msg := &rawFrame{}
		if err := clientStream.RecvMsg(msg); err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		if err := serverStream.SendMsg(msg); err != nil {
			return err
		}
	}
}

func extractRemoteAddr(ctx context.Context) string {
	p, ok := peer.FromContext(ctx)
	if !ok || p == nil || p.Addr == nil {
		return ""
	}
	return p.Addr.String()
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
