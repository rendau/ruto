package core

import (
	"context"
	"crypto/subtle"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/rendau/ruto/internal/constant"
	"github.com/rendau/ruto/pkg/proto/ruto_v1"
)

// gatewayMethods are called only by the gateways and have no REST route.
// Snapshot.Get hands out the whole config with its secrets unmasked, so none
// of them may answer a caller without the gateways' token.
var gatewayMethods = map[string]bool{
	ruto_v1.Snapshot_Get_FullMethodName:             true,
	ruto_v1.Gateway_Heartbeat_FullMethodName:        true,
	ruto_v1.Gateway_Subscribe_FullMethodName:        true,
	ruto_v1.BlockedClient_GetVersion_FullMethodName: true,
	ruto_v1.BlockedClient_GetActive_FullMethodName:  true,
}

var errGatewayToken = status.Error(codes.Unauthenticated, "gateway token required")

type gatewayTokenCheck struct {
	token []byte
}

// An empty token turns the check off: core and gateways may get the token in
// any order, see GATEWAY_TOKEN in docs/architecture.md.
func newGatewayTokenCheck(token string) *gatewayTokenCheck {
	return &gatewayTokenCheck{token: []byte(token)}
}

func (c *gatewayTokenCheck) allowed(ctx context.Context, fullMethod string) bool {
	if len(c.token) == 0 || !gatewayMethods[fullMethod] {
		return true
	}
	md, _ := metadata.FromIncomingContext(ctx)
	values := md.Get(constant.GatewayTokenMetadataKey)
	return len(values) == 1 && subtle.ConstantTimeCompare([]byte(values[0]), c.token) == 1
}

func (c *gatewayTokenCheck) unary() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if !c.allowed(ctx, info.FullMethod) {
			return nil, errGatewayToken
		}
		return handler(ctx, req)
	}
}

func (c *gatewayTokenCheck) stream() grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if !c.allowed(ss.Context(), info.FullMethod) {
			return errGatewayToken
		}
		return handler(srv, ss)
	}
}
