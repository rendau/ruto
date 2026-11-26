package core

import (
	"context"
	"errors"
	"io"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/rendau/ruto/internal/constant"
	"github.com/rendau/ruto/pkg/proto/ruto_v1"
)

type fakeSnapshotServer struct {
	ruto_v1.UnimplementedSnapshotServer
}

func (s *fakeSnapshotServer) Get(context.Context, *emptypb.Empty) (*ruto_v1.SnapshotResponse, error) {
	return &ruto_v1.SnapshotResponse{}, nil
}

func (s *fakeSnapshotServer) GetVersion(context.Context, *emptypb.Empty) (*ruto_v1.SnapshotVersion, error) {
	return &ruto_v1.SnapshotVersion{}, nil
}

type fakeGatewayServer struct {
	ruto_v1.UnimplementedGatewayServer
}

func (s *fakeGatewayServer) Subscribe(*ruto_v1.GatewaySubscribeRequest, grpc.ServerStreamingServer[ruto_v1.GatewayNotification]) error {
	return nil
}

func startTokenServer(t *testing.T, token string) *grpc.ClientConn {
	t.Helper()

	check := newGatewayTokenCheck(token)
	server := grpc.NewServer(
		grpc.ChainUnaryInterceptor(check.unary()),
		grpc.ChainStreamInterceptor(check.stream()),
	)
	ruto_v1.RegisterSnapshotServer(server, &fakeSnapshotServer{})
	ruto_v1.RegisterGatewayServer(server, &fakeGatewayServer{})

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	go func() { _ = server.Serve(lis) }()
	t.Cleanup(server.Stop)

	conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

func withToken(token string) context.Context {
	return metadata.AppendToOutgoingContext(context.Background(), constant.GatewayTokenMetadataKey, token)
}

func subscribeErr(ctx context.Context, conn *grpc.ClientConn) error {
	stream, err := ruto_v1.NewGatewayClient(conn).Subscribe(ctx, &ruto_v1.GatewaySubscribeRequest{GatewayId: "g"})
	if err != nil {
		return err
	}
	_, err = stream.Recv()
	if errors.Is(err, io.EOF) {
		return nil
	}
	return err
}

func TestGatewayToken(t *testing.T) {
	conn := startTokenServer(t, "s3cret")
	snapshot := ruto_v1.NewSnapshotClient(conn)

	_, err := snapshot.Get(context.Background(), &emptypb.Empty{})
	require.Equal(t, codes.Unauthenticated, status.Code(err))

	_, err = snapshot.Get(withToken("wrong"), &emptypb.Empty{})
	require.Equal(t, codes.Unauthenticated, status.Code(err))

	_, err = snapshot.Get(withToken("s3cret"), &emptypb.Empty{})
	require.NoError(t, err)

	// a method of the admin, not of the gateways, is left to the session check
	_, err = snapshot.GetVersion(context.Background(), &emptypb.Empty{})
	require.NoError(t, err)

	require.Equal(t, codes.Unauthenticated, status.Code(subscribeErr(context.Background(), conn)))
	require.NoError(t, subscribeErr(withToken("s3cret"), conn))
}

func TestGatewayTokenEmptyTurnsCheckOff(t *testing.T) {
	conn := startTokenServer(t, "")

	_, err := ruto_v1.NewSnapshotClient(conn).Get(context.Background(), &emptypb.Empty{})
	require.NoError(t, err)
	require.NoError(t, subscribeErr(context.Background(), conn))
}
