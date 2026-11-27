package core

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net"
	"net/netip"
	"os"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"

	configCore "github.com/rendau/ruto/internal/config/core"
	apiKeyService "github.com/rendau/ruto/internal/domain/apikey/service"
	sessionModel "github.com/rendau/ruto/internal/domain/session/model"
	sessionService "github.com/rendau/ruto/internal/domain/session/service"
	"github.com/rendau/ruto/internal/errs"
	"github.com/rendau/ruto/internal/infra/clientip"
	"github.com/rendau/ruto/pkg/proto/ruto_v1"
)

type GrpcServer struct {
	name   string
	server *grpc.Server
}

func NewGrpcServer(
	name string,
	sessionSvc *sessionService.Service,
	adminAuth AdminAuthI,
	apiKeyAuth ApiKeyAuthI,
	gatewayToken string,
	reflectionOn bool,
	register func(*grpc.Server),
) *GrpcServer {
	if gatewayToken == "" {
		slog.Warn(name + "-grpc-server: GATEWAY_TOKEN is empty, anyone reaching the grpc port reads the config with its secrets")
	}
	tokenCheck := newGatewayTokenCheck(gatewayToken)

	interceptors := make([]grpc.UnaryServerInterceptor, 0, 7)

	interceptors = append(interceptors, tokenCheck.unary())
	interceptors = append(interceptors, GrpcInterceptorCtxWithoutCancel())
	interceptors = append(interceptors, GrpcInterceptorClientIP())
	interceptors = append(interceptors, GrpcInterceptorSession(sessionSvc, adminAuth, apiKeyAuth))
	interceptors = append(interceptors, GrpcInterceptorRecovery())
	interceptors = append(interceptors, GrpcInterceptorError())

	server := grpc.NewServer(
		grpc.MaxSendMsgSize(math.MaxUint32),
		grpc.MaxRecvMsgSize(math.MaxUint32),
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             5 * time.Second,
			PermitWithoutStream: true,
		}),
		grpc.ChainUnaryInterceptor(interceptors...),
		grpc.ChainStreamInterceptor(tokenCheck.stream()),
	)

	if register != nil {
		register(server)
	}

	// reflection lists every method to whoever reaches the port: debugging only
	if reflectionOn {
		reflection.Register(server)
	}

	return &GrpcServer{
		name:   name,
		server: server,
	}
}

func (s *GrpcServer) Start() error {
	lis, err := net.Listen("tcp", ":"+strconv.Itoa(configCore.Conf.GrpcPort))
	if err != nil {
		return fmt.Errorf("failed to listen grpc: %w", err)
	}

	go func() {
		err = s.server.Serve(lis)
		if err != nil {
			slog.Error(s.name+"-grpc-server stopped", "error", err)
			os.Exit(1)
		}
	}()
	slog.Info(s.name + "-grpc-server started " + lis.Addr().String())
	return nil
}

func (s *GrpcServer) Stop() {
	// Prefer a graceful stop, but never hang shutdown: if some long-lived RPC
	// fails to drain in time, fall back to a hard stop.
	stopped := make(chan struct{})
	go func() {
		s.server.GracefulStop()
		close(stopped)
	}()

	select {
	case <-stopped:
	case <-time.After(10 * time.Second):
		slog.Warn(s.name + "-grpc-server graceful stop timed out, forcing stop")
		s.server.Stop()
		<-stopped
	}
}

func GrpcInterceptorCtxWithoutCancel() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		return handler(context.WithoutCancel(ctx), req)
	}
}

// AdminAuthI authenticates the token of an admin login.
type AdminAuthI interface {
	SessionFromToken(ctx context.Context, token string) (*sessionModel.Session, error)
}

// ApiKeyAuthI authenticates an api key (rtk_…) of the MCP server.
type ApiKeyAuthI interface {
	SessionFromKey(ctx context.Context, key string) (*sessionModel.Session, error)
}

// apiKeyMethods are the only API methods that accept an api key besides the
// admin login: services such as pulse block attackers with a key. Every
// other method keeps refusing keys, so a key never reads unmasked secrets or
// applies change requests. Callers with a key get the agent guards of
// usecase/blockedclient.
var apiKeyMethods = map[string]bool{
	ruto_v1.BlockedClient_List_FullMethodName:   true,
	ruto_v1.BlockedClient_Create_FullMethodName: true,
	ruto_v1.BlockedClient_Remove_FullMethodName: true,
}

func GrpcInterceptorSession(sessionSvc *sessionService.Service, adminAuth AdminAuthI, apiKeyAuth ApiKeyAuthI) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		var session *sessionModel.Session

		token := grpcExtractBearerToken(ctx)
		switch {
		case token == "":
		case strings.HasPrefix(token, apiKeyService.KeyPrefix):
			if apiKeyAuth != nil && apiKeyMethods[info.FullMethod] {
				keySession, keyErr := apiKeyAuth.SessionFromKey(ctx, token)
				if keyErr == nil {
					session = keySession
				} else {
					slog.Warn("api: api key rejected", "method", info.FullMethod, "error", keyErr)
				}
			}
		default:
			adminSession, adminErr := adminAuth.SessionFromToken(ctx, token)
			switch {
			case adminErr == nil:
				session = adminSession
			case !errors.Is(adminErr, errs.NotAuthorized):
				// a failed lookup is not a bad token: the admin UI must not
				// sign the user out over it
				slog.Error("api: admin token check", "method", info.FullMethod, "error", adminErr)
				return nil, status.Error(codes.Unavailable, errs.ServiceNA.Error())
			}
		}

		return handler(sessionSvc.WithContext(ctx, session), req)
	}
}

// clientIPMetadata carries the client address the grpc-gateway of this
// process resolved from the HTTP request. It is believed only from a loopback
// peer, and the grpc-gateway never passes it on from the client's headers.
const clientIPMetadata = "x-ruto-client-ip"

// GrpcInterceptorClientIP keeps the client address of the call in the context
// (see clientip.FromContext).
func GrpcInterceptorClientIP() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		return handler(clientip.WithContext(ctx, grpcClientIP(ctx)), req)
	}
}

func grpcClientIP(ctx context.Context) netip.Addr {
	p, ok := peer.FromContext(ctx)
	if !ok || p.Addr == nil {
		return netip.Addr{}
	}
	addrPort, err := netip.ParseAddrPort(p.Addr.String())
	if err != nil {
		return netip.Addr{}
	}
	ip := addrPort.Addr().Unmap()
	if !ip.IsLoopback() {
		return ip
	}

	md, _ := metadata.FromIncomingContext(ctx)
	if values := md.Get(clientIPMetadata); len(values) == 1 {
		if forwarded, parseErr := netip.ParseAddr(values[0]); parseErr == nil {
			return forwarded.Unmap()
		}
	}
	return ip
}

func grpcExtractBearerToken(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}

	values := md.Get("authorization")
	if len(values) == 0 {
		return ""
	}

	value := strings.TrimSpace(values[0])
	if value == "" {
		return ""
	}

	parts := strings.Fields(value)
	if len(parts) == 1 {
		return parts[0]
	}
	if len(parts) == 2 && strings.EqualFold(parts[0], "bearer") {
		return parts[1]
	}

	return ""
}

func GrpcInterceptorRecovery() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		defer func() {
			if recovered := recover(); recovered != nil {
				slog.Error(
					"Recovered from grpc panic",
					slog.Any("error", recovered),
					slog.String("fullMethod", info.FullMethod),
					slog.Any("recovery_stacktrace", string(debug.Stack())),
				)
				err = status.Error(codes.Internal, "internal server error")
			}
		}()

		return handler(ctx, req)
	}
}

func GrpcInterceptorError() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		h, err := handler(ctx, req)
		if err == nil {
			return h, nil
		}

		var ei *ruto_v1.ErrorRep
		errStr := err.Error()

		if fullErr, ok := errors.AsType[errs.ErrFull](err); ok {
			errCode := errs.ServiceNA.Error()
			if fullErr.Err != nil {
				errCode = fullErr.Err.Error()
			}
			ei = &ruto_v1.ErrorRep{
				Code:    errCode,
				Message: fullErr.Desc,
				Fields:  fullErr.Fields,
			}
		} else if baseErr, ok := errors.AsType[errs.Err](err); ok {
			ei = &ruto_v1.ErrorRep{
				Code:    baseErr.Error(),
				Message: errStr,
			}
		} else {
			ei = &ruto_v1.ErrorRep{
				Code:    errs.ServiceNA.Error(),
				Message: errStr,
			}
		}

		slog.Info(
			"GRPC handler error",
			slog.String("error", errStr),
			slog.String("method", info.FullMethod),
		)

		code := codes.InvalidArgument
		switch ei.Code {
		case errs.NotAuthorized.Error():
			code = codes.Unauthenticated
		case errs.NoPermission.Error():
			code = codes.PermissionDenied
		case errs.TooManyAttempts.Error():
			code = codes.ResourceExhausted
		}

		st := status.New(code, errStr)
		st, err = st.WithDetails(ei)
		if err != nil {
			slog.Error(
				"error while creating status with details",
				slog.String("error", errStr),
				slog.String("method", info.FullMethod),
			)
			st = status.New(code, errStr)
		}

		return h, st.Err()
	}
}
