package core

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net"
	"os"
	"runtime/debug"
	"strconv"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"

	configCore "github.com/rendau/ruto/internal/config/core"
	sessionModel "github.com/rendau/ruto/internal/domain/session/model"
	sessionService "github.com/rendau/ruto/internal/domain/session/service"
	"github.com/rendau/ruto/internal/errs"
	"github.com/rendau/ruto/pkg/proto/ruto_v1"
)

type GrpcServer struct {
	name   string
	server *grpc.Server
}

func NewGrpcServer(name string, sessionSvc *sessionService.Service, register func(*grpc.Server)) *GrpcServer {
	interceptors := make([]grpc.UnaryServerInterceptor, 0, 5)

	interceptors = append(interceptors, GrpcInterceptorCtxWithoutCancel())
	interceptors = append(interceptors, GrpcInterceptorSession(sessionSvc))
	interceptors = append(interceptors, GrpcInterceptorRecovery())
	interceptors = append(interceptors, GrpcInterceptorError())

	server := grpc.NewServer(
		grpc.MaxSendMsgSize(math.MaxUint32),
		grpc.MaxRecvMsgSize(math.MaxUint32),
		grpc.ChainUnaryInterceptor(interceptors...),
	)

	if register != nil {
		register(server)
	}

	reflection.Register(server)

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
	s.server.GracefulStop()
}

func GrpcInterceptorCtxWithoutCancel() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		return handler(context.WithoutCancel(ctx), req)
	}
}

func GrpcInterceptorSession(sessionSvc *sessionService.Service) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		var session *sessionModel.Session

		token := grpcExtractBearerToken(ctx)
		if token != "" {
			parsedSession, parseErr := sessionSvc.FromToken(token)
			if parseErr == nil && parsedSession != nil && parsedSession.Id != 0 {
				session = parsedSession
			}
		}

		return handler(sessionSvc.WithContext(ctx, session), req)
	}
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

		st := status.New(codes.InvalidArgument, errStr)
		st, err = st.WithDetails(ei)
		if err != nil {
			slog.Error(
				"error while creating status with details",
				slog.String("error", errStr),
				slog.String("method", info.FullMethod),
			)
			st = status.New(codes.InvalidArgument, errStr)
		}

		return h, st.Err()
	}
}
