package grpc

import (
	"context"
	"errors"
	"strconv"
	"strings"

	gogrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/rendau/ruto/internal/constant"
	domAppModel "github.com/rendau/ruto/internal/domain/app/model"
	domEndpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
	serviceAuth "github.com/rendau/ruto/internal/service/gw/service/auth"
	serviceAuthModel "github.com/rendau/ruto/internal/service/gw/service/auth/model"
	serviceLog "github.com/rendau/ruto/internal/service/gw/service/log"
	"github.com/rendau/ruto/internal/service/gw/service/logmask"
	serviceMetrics "github.com/rendau/ruto/internal/service/gw/service/metrics"
	"github.com/rendau/ruto/internal/service/gw/service/ratelimit"
)

type middleware func(gogrpc.StreamHandler) gogrpc.StreamHandler

// errOwnReject is a sentinel matched via errors.Is to recognize the gateway's
// own rejection — auth or rate limit — vs the same code returned by a backend.
var errOwnReject = errors.New("gateway own reject")

// errBlocked refuses a client on the block list; it is returned before the
// route's middlewares, so no log or metrics middleware sees it.
var errBlocked = status.Error(codes.PermissionDenied, "forbidden")

// authRejectError is the gateway's own auth rejection. It carries a proper gRPC
// Unauthenticated/PermissionDenied status for the client while remaining
// detectable via errors.Is(err, errOwnReject) in the logging layer.
type authRejectError struct {
	forbidden bool
}

func (e *authRejectError) Error() string {
	if e.forbidden {
		return "forbidden"
	}
	return "unauthorized"
}

func (e *authRejectError) GRPCStatus() *status.Status {
	if e.forbidden {
		return status.New(codes.PermissionDenied, e.Error())
	}
	return status.New(codes.Unauthenticated, e.Error())
}

func (e *authRejectError) Is(target error) bool { return target == errOwnReject }

// rateLimitError is the gateway's own rejection by a rate-limit rule.
type rateLimitError struct {
	retryAfter int
}

func (e *rateLimitError) Error() string {
	return "rate limit exceeded, retry after " + strconv.Itoa(e.retryAfter) + "s"
}

func (e *rateLimitError) GRPCStatus() *status.Status {
	return status.New(codes.ResourceExhausted, e.Error())
}

func (e *rateLimitError) Is(target error) bool { return target == errOwnReject }

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
			if result := service.Check(authReq); result != serviceAuthModel.Allowed {
				return &authRejectError{forbidden: result.Forbidden()}
			}

			return next(srv, stream)
		}
	}
}

// newRateLimitMiddleware goes before auth, so attempts with wrong credentials
// are counted too (see NewRateLimit of the http handler for how). gRPC has no Retry-After: the wait goes in the status
// message and in the "retry-after" trailer, in seconds.
func newRateLimitMiddleware(limiter *ratelimit.Endpoint) middleware {
	if limiter == nil {
		return func(next gogrpc.StreamHandler) gogrpc.StreamHandler { return next }
	}

	return func(next gogrpc.StreamHandler) gogrpc.StreamHandler {
		return func(srv any, stream gogrpc.ServerStream) error {
			md := metadataFromContext(stream.Context())

			ok, retryAfter := limiter.Check(ratelimit.Request{
				RemoteAddr:    remoteAddrFromContext(stream.Context()),
				Header:        headersFromMetadata(md),
				Authorization: strings.TrimSpace(metadataFirstValue(md, metadataHeaderAuthorization)),
			})
			if ok {
				return next(srv, stream)
			}

			seconds := ratelimit.RetryAfterSeconds(retryAfter)
			stream.SetTrailer(metadata.Pairs(metadataHeaderRetryAfter, strconv.Itoa(seconds)))
			return &rateLimitError{retryAfter: seconds}
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

	if ep.Logging.EffectiveLevel() == constant.LoggingLevelNone {
		return func(next gogrpc.StreamHandler) gogrpc.StreamHandler { return next }
	}

	logHeaders := ep.Logging.Headers
	sensitive := logmask.BuildSensitiveKeySet(ep)

	return func(next gogrpc.StreamHandler) gogrpc.StreamHandler {
		return func(srv any, stream gogrpc.ServerStream) (err error) {
			service.Serve(func() ([]any, string, error, bool) {
				fields := []any{
					"app_name", app.Name,
					"grpc_service", ep.Grpc.Service,
					"grpc_method", ep.Grpc.Method,
					"grpc_path", ep.Grpc.Path,
				}
				if logHeaders {
					md := metadataFromContext(stream.Context())
					fields = append(fields, "metadata", logmask.MaskValues(md, sensitive))
				}

				err = next(srv, stream)

				return fields, statusLabelFromError(err), err, errors.Is(err, errOwnReject)
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
