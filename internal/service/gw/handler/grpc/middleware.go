package grpc

import (
	"context"
	"errors"

	gogrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	domAppModel "github.com/rendau/ruto/internal/domain/app/model"
	domEndpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
	serviceAuth "github.com/rendau/ruto/internal/service/gw/service/auth"
	serviceAuthModel "github.com/rendau/ruto/internal/service/gw/service/auth/model"
	serviceLog "github.com/rendau/ruto/internal/service/gw/service/log"
	serviceMetrics "github.com/rendau/ruto/internal/service/gw/service/metrics"
)

type middleware func(gogrpc.StreamHandler) gogrpc.StreamHandler

func chain(h gogrpc.StreamHandler, middlewares ...middleware) gogrpc.StreamHandler {
	for i := len(middlewares) - 1; i >= 0; i-- {
		h = middlewares[i](h)
	}
	return h
}

func newAuthMiddleware(service *serviceAuth.Service) middleware {
	if service == nil {
		return func(next gogrpc.StreamHandler) gogrpc.StreamHandler { return next }
	}

	return func(next gogrpc.StreamHandler) gogrpc.StreamHandler {
		return func(srv any, stream gogrpc.ServerStream) error {
			md := metadataFromContext(stream.Context())

			authReq := serviceAuthModel.NewAuthRequest()
			authReq.SetHttpHeader(headersFromMetadata(md))
			authReq.SetRemoteAddr(remoteAddrFromContext(stream.Context()))
			if !service.Check(authReq) {
				return status.Error(codes.Unauthenticated, "unauthorized")
			}

			return next(srv, stream)
		}
	}
}

func newRequestLogMiddleware(
	service *serviceLog.Service,
	app *domAppModel.App,
	ep *domEndpointModel.Endpoint,
) middleware {
	if service == nil {
		return func(next gogrpc.StreamHandler) gogrpc.StreamHandler { return next }
	}

	return func(next gogrpc.StreamHandler) gogrpc.StreamHandler {
		return func(srv any, stream gogrpc.ServerStream) (err error) {
			service.Serve(func() ([]any, string, error) {
				err = next(srv, stream)
				return []any{
					"app_name", app.Name,
					"grpc_service", ep.Grpc.Service,
					"grpc_method", ep.Grpc.Method,
					"grpc_path", ep.Grpc.Path,
				}, statusLabelFromError(err), err
			})
			return err
		}
	}
}

func newMetricsMiddleware(service *serviceMetrics.Service) middleware {
	if service == nil {
		return func(next gogrpc.StreamHandler) gogrpc.StreamHandler { return next }
	}

	return func(next gogrpc.StreamHandler) gogrpc.StreamHandler {
		return func(srv any, stream gogrpc.ServerStream) (err error) {
			service.Serve(func() string {
				err = next(srv, stream)
				return statusLabelFromError(err)
			})
			return err
		}
	}
}

func statusLabelFromError(err error) string {
	if err == nil {
		return codes.OK.String()
	}
	if st, ok := status.FromError(err); ok {
		return st.Code().String()
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return codes.DeadlineExceeded.String()
	}
	return codes.Internal.String()
}
