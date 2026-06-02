package grpc

import (
	"context"
	"fmt"
	"io"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	gogrpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	grpcTesting "google.golang.org/grpc/interop/grpc_testing"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/reflection"
	reflectionv1alpha "google.golang.org/grpc/reflection/grpc_reflection_v1"
	"google.golang.org/grpc/status"

	appModel "github.com/rendau/ruto/internal/domain/app/model"
	authModel "github.com/rendau/ruto/internal/domain/auth/model"
	endpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
	rootModel "github.com/rendau/ruto/internal/domain/root/model"
	gwGrpcServer "github.com/rendau/ruto/internal/service/gw/server/grpc"
)

func TestProxy_UnaryCall(t *testing.T) {
	t.Parallel()

	backendAddr, backendStop := runBackendTestService(t)
	defer backendStop()

	svc, err := New(buildSnapshotForBackend(t, backendAddr), false)
	require.NoError(t, err)

	gatewayAddr, gatewayStop := runGatewayProxyServer(t, svc)
	defer gatewayStop()

	conn, err := gogrpc.NewClient(gatewayAddr, gogrpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	client := grpcTesting.NewTestServiceClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ctx = metadata.AppendToOutgoingContext(ctx, metadataHeaderAppName, "backend-test")

	resp, err := client.UnaryCall(ctx, &grpcTesting.SimpleRequest{
		Payload: &grpcTesting.Payload{
			Body: []byte("ping-unary"),
		},
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotNil(t, resp.Payload)
	require.Equal(t, []byte("backend:ping-unary"), resp.Payload.Body)
}

func TestProxy_BidiStreamingCall(t *testing.T) {
	t.Parallel()

	backendAddr, backendStop := runBackendTestService(t)
	defer backendStop()

	svc, err := New(buildSnapshotForBackend(t, backendAddr), false)
	require.NoError(t, err)

	gatewayAddr, gatewayStop := runGatewayProxyServer(t, svc)
	defer gatewayStop()

	conn, err := gogrpc.NewClient(gatewayAddr, gogrpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	client := grpcTesting.NewTestServiceClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ctx = metadata.AppendToOutgoingContext(ctx, metadataHeaderAppName, "backend-test")

	stream, err := client.FullDuplexCall(ctx)
	require.NoError(t, err)

	sendBodies := [][]byte{
		[]byte("stream-1"),
		[]byte("stream-2"),
	}
	for _, body := range sendBodies {
		err = stream.Send(&grpcTesting.StreamingOutputCallRequest{
			Payload: &grpcTesting.Payload{Body: body},
		})
		require.NoError(t, err)

		rep, recvErr := stream.Recv()
		require.NoError(t, recvErr)
		require.NotNil(t, rep)
		require.NotNil(t, rep.Payload)
		require.Equal(t, append([]byte("backend:"), body...), rep.Payload.Body)
	}

	require.NoError(t, stream.CloseSend())

	_, err = stream.Recv()
	require.ErrorIs(t, err, io.EOF)
}

func TestProxy_MissingAppMetadata(t *testing.T) {
	t.Parallel()

	backendAddr, backendStop := runBackendTestService(t)
	defer backendStop()

	svc, err := New(buildSnapshotForBackend(t, backendAddr), false)
	require.NoError(t, err)

	gatewayAddr, gatewayStop := runGatewayProxyServer(t, svc)
	defer gatewayStop()

	conn, err := gogrpc.NewClient(gatewayAddr, gogrpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	client := grpcTesting.NewTestServiceClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = client.UnaryCall(ctx, &grpcTesting.SimpleRequest{
		Payload: &grpcTesting.Payload{
			Body: []byte("ping-unary"),
		},
	})
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	require.Equal(t, codes.NotFound, st.Code())
}

func TestProxy_DuplicateMethodResolvedByAppName(t *testing.T) {
	t.Parallel()

	backendAddr1, backendStop1 := runBackendTestServiceWithPrefix(t, "backend-1:")
	defer backendStop1()
	backendAddr2, backendStop2 := runBackendTestServiceWithPrefix(t, "backend-2:")
	defer backendStop2()

	svc, err := New(buildSnapshotForTwoBackends(t, backendAddr1, backendAddr2), false)
	require.NoError(t, err)

	gatewayAddr, gatewayStop := runGatewayProxyServer(t, svc)
	defer gatewayStop()

	conn, err := gogrpc.NewClient(gatewayAddr, gogrpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	client := grpcTesting.NewTestServiceClient(conn)

	ctx1, cancel1 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel1()
	ctx1 = metadata.AppendToOutgoingContext(ctx1, metadataHeaderAppName, "backend-test-1")

	rep1, err := client.UnaryCall(ctx1, &grpcTesting.SimpleRequest{
		Payload: &grpcTesting.Payload{
			Body: []byte("ping"),
		},
	})
	require.NoError(t, err)
	require.Equal(t, []byte("backend-1:ping"), rep1.GetPayload().GetBody())

	ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel2()
	ctx2 = metadata.AppendToOutgoingContext(ctx2, metadataHeaderAppName, "backend-test-2")

	rep2, err := client.UnaryCall(ctx2, &grpcTesting.SimpleRequest{
		Payload: &grpcTesting.Payload{
			Body: []byte("ping"),
		},
	})
	require.NoError(t, err)
	require.Equal(t, []byte("backend-2:ping"), rep2.GetPayload().GetBody())
}

func TestReflection_ListServicesFilteredByRegisteredEndpoints(t *testing.T) {
	t.Parallel()

	backendAddr, backendStop := runBackendTestService(t)
	defer backendStop()

	svc, err := New(buildSnapshotForBackend(t, backendAddr), false)
	require.NoError(t, err)

	gatewayAddr, gatewayStop := runGatewayProxyServer(t, svc)
	defer gatewayStop()

	conn, err := gogrpc.NewClient(gatewayAddr, gogrpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ctx = metadata.AppendToOutgoingContext(ctx, metadataHeaderAppName, "backend-test")

	stream, err := reflectionv1alpha.NewServerReflectionClient(conn).ServerReflectionInfo(ctx)
	require.NoError(t, err)
	require.NoError(t, stream.Send(&reflectionv1alpha.ServerReflectionRequest{
		MessageRequest: &reflectionv1alpha.ServerReflectionRequest_ListServices{
			ListServices: "",
		},
	}))
	require.NoError(t, stream.CloseSend())

	resp, err := stream.Recv()
	require.NoError(t, err)
	require.NotNil(t, resp.GetListServicesResponse())
	require.Equal(t, []*reflectionv1alpha.ServiceResponse{
		{Name: "grpc.testing.TestService"},
	}, resp.GetListServicesResponse().GetService())
}

func TestReflection_FileContainingRegisteredSymbolProxiedToBackend(t *testing.T) {
	t.Parallel()

	backendAddr, backendStop := runBackendTestService(t)
	defer backendStop()

	svc, err := New(buildSnapshotForBackend(t, backendAddr), false)
	require.NoError(t, err)

	gatewayAddr, gatewayStop := runGatewayProxyServer(t, svc)
	defer gatewayStop()

	conn, err := gogrpc.NewClient(gatewayAddr, gogrpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ctx = metadata.AppendToOutgoingContext(ctx, metadataHeaderAppName, "backend-test")

	stream, err := reflectionv1alpha.NewServerReflectionClient(conn).ServerReflectionInfo(ctx)
	require.NoError(t, err)
	require.NoError(t, stream.Send(&reflectionv1alpha.ServerReflectionRequest{
		MessageRequest: &reflectionv1alpha.ServerReflectionRequest_FileContainingSymbol{
			FileContainingSymbol: "grpc.testing.TestService",
		},
	}))
	require.NoError(t, stream.CloseSend())

	resp, err := stream.Recv()
	require.NoError(t, err)
	require.Nil(t, resp.GetErrorResponse())
	require.NotNil(t, resp.GetFileDescriptorResponse())
	require.NotEmpty(t, resp.GetFileDescriptorResponse().GetFileDescriptorProto())
}

func TestReflection_FileContainingUnregisteredSymbolRejected(t *testing.T) {
	t.Parallel()

	backendAddr, backendStop := runBackendTestService(t)
	defer backendStop()

	svc, err := New(buildSnapshotForBackend(t, backendAddr), false)
	require.NoError(t, err)

	gatewayAddr, gatewayStop := runGatewayProxyServer(t, svc)
	defer gatewayStop()

	conn, err := gogrpc.NewClient(gatewayAddr, gogrpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ctx = metadata.AppendToOutgoingContext(ctx, metadataHeaderAppName, "backend-test")

	stream, err := reflectionv1alpha.NewServerReflectionClient(conn).ServerReflectionInfo(ctx)
	require.NoError(t, err)
	require.NoError(t, stream.Send(&reflectionv1alpha.ServerReflectionRequest{
		MessageRequest: &reflectionv1alpha.ServerReflectionRequest_FileContainingSymbol{
			FileContainingSymbol: "grpc.testing.UnregisteredService",
		},
	}))
	require.NoError(t, stream.CloseSend())

	resp, err := stream.Recv()
	require.NoError(t, err)
	require.NotNil(t, resp.GetErrorResponse())
	require.Equal(t, int32(codes.NotFound), resp.GetErrorResponse().GetErrorCode())
}

type testBackendServer struct {
	grpcTesting.UnimplementedTestServiceServer
	prefix string
}

func (s *testBackendServer) UnaryCall(_ context.Context, req *grpcTesting.SimpleRequest) (*grpcTesting.SimpleResponse, error) {
	inBody := []byte{}
	if req.GetPayload() != nil {
		inBody = req.GetPayload().GetBody()
	}
	return &grpcTesting.SimpleResponse{
		Payload: &grpcTesting.Payload{
			Body: append([]byte(s.prefix), inBody...),
		},
	}, nil
}

func (s *testBackendServer) FullDuplexCall(stream gogrpc.BidiStreamingServer[grpcTesting.StreamingOutputCallRequest, grpcTesting.StreamingOutputCallResponse]) error {
	for {
		req, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		inBody := []byte{}
		if req.GetPayload() != nil {
			inBody = req.GetPayload().GetBody()
		}

		if sendErr := stream.Send(&grpcTesting.StreamingOutputCallResponse{
			Payload: &grpcTesting.Payload{
				Body: append([]byte(s.prefix), inBody...),
			},
		}); sendErr != nil {
			return sendErr
		}
	}
}

func runBackendTestService(t *testing.T) (string, func()) {
	return runBackendTestServiceWithPrefix(t, "backend:")
}

func runBackendTestServiceWithPrefix(t *testing.T, prefix string) (string, func()) {
	t.Helper()

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	server := gogrpc.NewServer()
	grpcTesting.RegisterTestServiceServer(server, &testBackendServer{prefix: prefix})
	reflection.Register(server)

	go func() {
		_ = server.Serve(lis)
	}()

	return lis.Addr().String(), func() {
		server.GracefulStop()
		_ = lis.Close()
	}
}

func runGatewayProxyServer(t *testing.T, svc *Service) (string, func()) {
	t.Helper()

	port := getFreeTCPPort(t)

	server := gwGrpcServer.New(port)
	server.SetHandler(svc)
	server.Start()

	return "127.0.0.1:" + strconv.Itoa(port), func() {
		_ = server.Stop(3 * time.Second)
	}
}

func getFreeTCPPort(t *testing.T) int {
	t.Helper()

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = lis.Close() }()

	addr, ok := lis.Addr().(*net.TCPAddr)
	require.True(t, ok)
	return addr.Port
}

func buildSnapshotForBackend(t *testing.T, backendAddr string) *rootModel.Root {
	t.Helper()

	host, portRaw, err := net.SplitHostPort(backendAddr)
	require.NoError(t, err)

	port, err := strconv.Atoi(portRaw)
	require.NoError(t, err)

	snapshot := &rootModel.Root{
		Auth: authModel.Auth{
			Enabled: false,
			Mode:    "extend",
		},
		Apps: []*appModel.App{
			{
				Id:         "app-1",
				Active:     true,
				PathPrefix: "/svc",
				Name:       "backend-test",
				Backend: appModel.AppBackend{
					Url: fmt.Sprintf("http://%s:%d", host, port),
				},
				GrpcPort: port,
				Auth: authModel.Auth{
					Enabled: false,
					Mode:    "extend",
				},
				Endpoints: []*endpointModel.Endpoint{
					{
						Id:     "ep-unary",
						AppId:  "app-1",
						Active: true,
						Type:   endpointModel.TypeGRPC,
						Grpc: endpointModel.Grpc{
							Service: "grpc.testing.TestService",
							Method:  "UnaryCall",
							Path:    "/grpc.testing.TestService/UnaryCall",
						},
						Auth: authModel.Auth{
							Enabled: false,
							Mode:    "extend",
						},
					},
					{
						Id:     "ep-bidi",
						AppId:  "app-1",
						Active: true,
						Type:   endpointModel.TypeGRPC,
						Grpc: endpointModel.Grpc{
							Service: "grpc.testing.TestService",
							Method:  "FullDuplexCall",
							Path:    "/grpc.testing.TestService/FullDuplexCall",
						},
						Auth: authModel.Auth{
							Enabled: false,
							Mode:    "extend",
						},
					},
				},
			},
		},
	}

	require.NoError(t, snapshot.Normalize())
	return snapshot
}

func buildSnapshotForTwoBackends(t *testing.T, backendAddr1, backendAddr2 string) *rootModel.Root {
	t.Helper()

	host1, portRaw1, err := net.SplitHostPort(backendAddr1)
	require.NoError(t, err)
	port1, err := strconv.Atoi(portRaw1)
	require.NoError(t, err)

	host2, portRaw2, err := net.SplitHostPort(backendAddr2)
	require.NoError(t, err)
	port2, err := strconv.Atoi(portRaw2)
	require.NoError(t, err)

	snapshot := &rootModel.Root{
		Auth: authModel.Auth{
			Enabled: false,
			Mode:    "extend",
		},
		Apps: []*appModel.App{
			{
				Id:         "app-1",
				Active:     true,
				PathPrefix: "/svc1",
				Name:       "backend-test-1",
				Backend: appModel.AppBackend{
					Url: fmt.Sprintf("http://%s:%d", host1, port1),
				},
				GrpcPort: port1,
				Auth: authModel.Auth{
					Enabled: false,
					Mode:    "extend",
				},
				Endpoints: []*endpointModel.Endpoint{
					{
						Id:     "ep-unary-1",
						AppId:  "app-1",
						Active: true,
						Type:   endpointModel.TypeGRPC,
						Grpc: endpointModel.Grpc{
							Service: "grpc.testing.TestService",
							Method:  "UnaryCall",
							Path:    "/grpc.testing.TestService/UnaryCall",
						},
						Auth: authModel.Auth{
							Enabled: false,
							Mode:    "extend",
						},
					},
				},
			},
			{
				Id:         "app-2",
				Active:     true,
				PathPrefix: "/svc2",
				Name:       "backend-test-2",
				Backend: appModel.AppBackend{
					Url: fmt.Sprintf("http://%s:%d", host2, port2),
				},
				GrpcPort: port2,
				Auth: authModel.Auth{
					Enabled: false,
					Mode:    "extend",
				},
				Endpoints: []*endpointModel.Endpoint{
					{
						Id:     "ep-unary-2",
						AppId:  "app-2",
						Active: true,
						Type:   endpointModel.TypeGRPC,
						Grpc: endpointModel.Grpc{
							Service: "grpc.testing.TestService",
							Method:  "UnaryCall",
							Path:    "/grpc.testing.TestService/UnaryCall",
						},
						Auth: authModel.Auth{
							Enabled: false,
							Mode:    "extend",
						},
					},
				},
			},
		},
	}

	require.NoError(t, snapshot.Normalize())
	return snapshot
}
