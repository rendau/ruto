package grpc

import (
	"context"
	"fmt"
	"io"
	"net"
	"slices"
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
	reflectionv1 "google.golang.org/grpc/reflection/grpc_reflection_v1"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/types/descriptorpb"

	appModel "github.com/rendau/ruto/internal/domain/app/model"
	authModel "github.com/rendau/ruto/internal/domain/auth/model"
	endpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
	rootModel "github.com/rendau/ruto/internal/domain/root/model"
	varsModel "github.com/rendau/ruto/internal/domain/vars/model"
	gwGrpcServer "github.com/rendau/ruto/internal/service/gw/server/grpc"
)

func TestProxy_UnaryCall(t *testing.T) {

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

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
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

func TestProxy_BackendHeaders(t *testing.T) {
	backendServer := &testBackendServer{
		prefix:     "backend:",
		metadataCh: make(chan metadata.MD, 1),
	}
	backendAddr, backendStop := runBackendTestServiceWithServer(t, backendServer)
	defer backendStop()

	snapshot := buildSnapshotForBackend(t, backendAddr)
	snapshot.Apps[0].Backend.Headers = varsModel.Vars{
		"x-app-token": "app-token",
		"x-shared":    "app",
	}
	snapshot.Apps[0].Endpoints[0].Backend.Headers = varsModel.Vars{
		"x-endpoint-token": "endpoint-token",
		"x-shared":         "endpoint",
	}
	snapshot.InheritDown()

	svc, err := New(snapshot, false)
	require.NoError(t, err)

	gatewayAddr, gatewayStop := runGatewayProxyServer(t, svc)
	defer gatewayStop()

	conn, err := gogrpc.NewClient(gatewayAddr, gogrpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	client := grpcTesting.NewTestServiceClient(conn)

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	ctx = metadata.AppendToOutgoingContext(
		ctx,
		metadataHeaderAppName, "backend-test",
		"x-client", "client",
		"x-shared", "client",
	)

	_, err = client.UnaryCall(ctx, &grpcTesting.SimpleRequest{
		Payload: &grpcTesting.Payload{
			Body: []byte("ping-unary"),
		},
	})
	require.NoError(t, err)

	select {
	case md := <-backendServer.metadataCh:
		require.Equal(t, []string{"app-token"}, md.Get("x-app-token"))
		require.Equal(t, []string{"endpoint-token"}, md.Get("x-endpoint-token"))
		require.Equal(t, []string{"endpoint"}, md.Get("x-shared"))
		require.Equal(t, []string{"client"}, md.Get("x-client"))
	case <-time.After(5 * time.Second):
		require.FailNow(t, "backend metadata was not captured")
	}
}

func TestProxy_BidiStreamingCall(t *testing.T) {

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

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
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
		require.Equal(t, slices.Concat([]byte("backend:"), body), rep.Payload.Body)
	}

	require.NoError(t, stream.CloseSend())

	_, err = stream.Recv()
	require.ErrorIs(t, err, io.EOF)
}

func TestProxy_MissingAppMetadata(t *testing.T) {

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

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
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

	ctx1, cancel1 := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel1()
	ctx1 = metadata.AppendToOutgoingContext(ctx1, metadataHeaderAppName, "backend-test-1")

	rep1, err := client.UnaryCall(ctx1, &grpcTesting.SimpleRequest{
		Payload: &grpcTesting.Payload{
			Body: []byte("ping"),
		},
	})
	require.NoError(t, err)
	require.Equal(t, []byte("backend-1:ping"), rep1.GetPayload().GetBody())

	ctx2, cancel2 := context.WithTimeout(t.Context(), 5*time.Second)
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

	backendAddr, backendStop := runBackendTestService(t)
	defer backendStop()

	svc, err := New(buildSnapshotForBackend(t, backendAddr), false)
	require.NoError(t, err)

	gatewayAddr, gatewayStop := runGatewayProxyServer(t, svc)
	defer gatewayStop()

	conn, err := gogrpc.NewClient(gatewayAddr, gogrpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	ctx = metadata.AppendToOutgoingContext(ctx, metadataHeaderAppName, "backend-test")

	stream, err := reflectionv1.NewServerReflectionClient(conn).ServerReflectionInfo(ctx)
	require.NoError(t, err)
	require.NoError(t, stream.Send(&reflectionv1.ServerReflectionRequest{
		MessageRequest: &reflectionv1.ServerReflectionRequest_ListServices{
			ListServices: "",
		},
	}))
	require.NoError(t, stream.CloseSend())

	resp, err := stream.Recv()
	require.NoError(t, err)
	require.NotNil(t, resp.GetListServicesResponse())
	require.Equal(t, []*reflectionv1.ServiceResponse{
		{Name: "grpc.testing.TestService"},
	}, resp.GetListServicesResponse().GetService())
}

func TestReflection_FileContainingRegisteredSymbolProxiedToBackend(t *testing.T) {

	backendAddr, backendStop := runBackendTestService(t)
	defer backendStop()

	svc, err := New(buildSnapshotForBackend(t, backendAddr), false)
	require.NoError(t, err)

	gatewayAddr, gatewayStop := runGatewayProxyServer(t, svc)
	defer gatewayStop()

	conn, err := gogrpc.NewClient(gatewayAddr, gogrpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	ctx = metadata.AppendToOutgoingContext(ctx, metadataHeaderAppName, "backend-test")

	stream, err := reflectionv1.NewServerReflectionClient(conn).ServerReflectionInfo(ctx)
	require.NoError(t, err)
	require.NoError(t, stream.Send(&reflectionv1.ServerReflectionRequest{
		MessageRequest: &reflectionv1.ServerReflectionRequest_FileContainingSymbol{
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

func TestReflection_FileDescriptorMethodsFilteredByRegisteredEndpoints(t *testing.T) {

	backendAddr, backendStop := runBackendTestService(t)
	defer backendStop()

	svc, err := New(buildSnapshotForBackendUnaryOnly(t, backendAddr), false)
	require.NoError(t, err)

	gatewayAddr, gatewayStop := runGatewayProxyServer(t, svc)
	defer gatewayStop()

	conn, err := gogrpc.NewClient(gatewayAddr, gogrpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	ctx = metadata.AppendToOutgoingContext(ctx, metadataHeaderAppName, "backend-test")

	stream, err := reflectionv1.NewServerReflectionClient(conn).ServerReflectionInfo(ctx)
	require.NoError(t, err)
	require.NoError(t, stream.Send(&reflectionv1.ServerReflectionRequest{
		MessageRequest: &reflectionv1.ServerReflectionRequest_FileContainingSymbol{
			FileContainingSymbol: "grpc.testing.TestService",
		},
	}))
	require.NoError(t, stream.CloseSend())

	resp, err := stream.Recv()
	require.NoError(t, err)
	require.Nil(t, resp.GetErrorResponse())

	methods := reflectionResponseServiceMethods(t, resp, "grpc.testing.TestService")
	require.Equal(t, []string{"UnaryCall"}, methods)
}

func TestReflection_FileContainingUnregisteredMethodRejected(t *testing.T) {

	backendAddr, backendStop := runBackendTestService(t)
	defer backendStop()

	svc, err := New(buildSnapshotForBackendUnaryOnly(t, backendAddr), false)
	require.NoError(t, err)

	gatewayAddr, gatewayStop := runGatewayProxyServer(t, svc)
	defer gatewayStop()

	conn, err := gogrpc.NewClient(gatewayAddr, gogrpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	ctx = metadata.AppendToOutgoingContext(ctx, metadataHeaderAppName, "backend-test")

	stream, err := reflectionv1.NewServerReflectionClient(conn).ServerReflectionInfo(ctx)
	require.NoError(t, err)
	require.NoError(t, stream.Send(&reflectionv1.ServerReflectionRequest{
		MessageRequest: &reflectionv1.ServerReflectionRequest_FileContainingSymbol{
			FileContainingSymbol: "grpc.testing.TestService.FullDuplexCall",
		},
	}))
	require.NoError(t, stream.CloseSend())

	resp, err := stream.Recv()
	require.NoError(t, err)
	require.NotNil(t, resp.GetErrorResponse())
	require.Equal(t, int32(codes.NotFound), resp.GetErrorResponse().GetErrorCode())
}

func TestReflection_FileContainingUnregisteredSymbolRejected(t *testing.T) {

	backendAddr, backendStop := runBackendTestService(t)
	defer backendStop()

	svc, err := New(buildSnapshotForBackend(t, backendAddr), false)
	require.NoError(t, err)

	gatewayAddr, gatewayStop := runGatewayProxyServer(t, svc)
	defer gatewayStop()

	conn, err := gogrpc.NewClient(gatewayAddr, gogrpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	ctx = metadata.AppendToOutgoingContext(ctx, metadataHeaderAppName, "backend-test")

	stream, err := reflectionv1.NewServerReflectionClient(conn).ServerReflectionInfo(ctx)
	require.NoError(t, err)
	require.NoError(t, stream.Send(&reflectionv1.ServerReflectionRequest{
		MessageRequest: &reflectionv1.ServerReflectionRequest_FileContainingSymbol{
			FileContainingSymbol: "grpc.testing.UnregisteredService",
		},
	}))
	require.NoError(t, stream.CloseSend())

	resp, err := stream.Recv()
	require.NoError(t, err)
	require.NotNil(t, resp.GetErrorResponse())
	require.Equal(t, int32(codes.NotFound), resp.GetErrorResponse().GetErrorCode())
}

func TestReflection_FilterPreservesDependencyTypesFromUnregisteredServiceFiles(t *testing.T) {
	depFile := &descriptorpb.FileDescriptorProto{
		Name:    new("nsi_v1/product_group.proto"),
		Package: new("nsi_v1"),
		Syntax:  new("proto3"),
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: new("ProductGroupMain"),
			},
		},
		Service: []*descriptorpb.ServiceDescriptorProto{
			{
				Name: new("ProductGroup"),
				Method: []*descriptorpb.MethodDescriptorProto{
					{
						Name:       new("List"),
						InputType:  new(".nsi_v1.ProductGroupMain"),
						OutputType: new(".nsi_v1.ProductGroupMain"),
					},
				},
			},
		},
	}
	mainFile := &descriptorpb.FileDescriptorProto{
		Name:       new("nsi_v1/product.proto"),
		Package:    new("nsi_v1"),
		Syntax:     new("proto3"),
		Dependency: []string{depFile.GetName()},
		MessageType: []*descriptorpb.DescriptorProto{
			{
				Name: new("ProductMain"),
				Field: []*descriptorpb.FieldDescriptorProto{
					{
						Name:     new("group"),
						Number:   new(int32(1)),
						Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
						Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
						TypeName: new(".nsi_v1.ProductGroupMain"),
					},
				},
			},
		},
		Service: []*descriptorpb.ServiceDescriptorProto{
			{
				Name: new("Product"),
				Method: []*descriptorpb.MethodDescriptorProto{
					{
						Name:       new("Get"),
						InputType:  new(".nsi_v1.ProductMain"),
						OutputType: new(".nsi_v1.ProductMain"),
					},
				},
			},
		},
	}

	mainRaw, err := proto.Marshal(mainFile)
	require.NoError(t, err)
	depRaw, err := proto.Marshal(depFile)
	require.NoError(t, err)

	fileResp := &reflectionv1.FileDescriptorResponse{
		FileDescriptorProto: [][]byte{mainRaw, depRaw},
	}
	rt := &reflectionRoute{
		methods: map[string]map[string]struct{}{
			"nsi_v1.Product": {
				"Get": {},
			},
		},
	}

	require.NoError(t, rt.filterFileDescriptorResponse(fileResp))
	require.Len(t, fileResp.GetFileDescriptorProto(), 2)

	filteredSet := &descriptorpb.FileDescriptorSet{}
	for _, raw := range fileResp.GetFileDescriptorProto() {
		file := &descriptorpb.FileDescriptorProto{}
		require.NoError(t, proto.Unmarshal(raw, file))
		filteredSet.File = append(filteredSet.File, file)

		if file.GetName() == depFile.GetName() {
			require.Equal(t, []string{"ProductGroupMain"}, descriptorMessageNames(file))
			require.Empty(t, file.GetService())
		}
	}

	_, err = protodesc.NewFiles(filteredSet)
	require.NoError(t, err)
}

func reflectionResponseServiceMethods(
	t *testing.T,
	resp *reflectionv1.ServerReflectionResponse,
	serviceName string,
) []string {
	t.Helper()

	fileResp := resp.GetFileDescriptorResponse()
	require.NotNil(t, fileResp)

	methods := make([]string, 0)
	for _, raw := range fileResp.GetFileDescriptorProto() {
		file := &descriptorpb.FileDescriptorProto{}
		require.NoError(t, proto.Unmarshal(raw, file))

		for _, svc := range file.GetService() {
			if joinProtoName(file.GetPackage(), svc.GetName()) != serviceName {
				continue
			}
			for _, method := range svc.GetMethod() {
				methods = append(methods, method.GetName())
			}
		}
	}
	slices.Sort(methods)
	return methods
}

func descriptorMessageNames(file *descriptorpb.FileDescriptorProto) []string {
	names := make([]string, 0, len(file.GetMessageType()))
	for _, msg := range file.GetMessageType() {
		names = append(names, msg.GetName())
	}
	slices.Sort(names)
	return names
}

type testBackendServer struct {
	grpcTesting.UnimplementedTestServiceServer
	prefix     string
	metadataCh chan metadata.MD
}

func (s *testBackendServer) UnaryCall(ctx context.Context, req *grpcTesting.SimpleRequest) (*grpcTesting.SimpleResponse, error) {
	s.captureMetadata(ctx)

	inBody := []byte(nil)
	if req.GetPayload() != nil {
		inBody = req.GetPayload().GetBody()
	}
	return &grpcTesting.SimpleResponse{
		Payload: &grpcTesting.Payload{
			Body: slices.Concat([]byte(s.prefix), inBody),
		},
	}, nil
}

func (s *testBackendServer) captureMetadata(ctx context.Context) {
	if s.metadataCh == nil {
		return
	}
	select {
	case s.metadataCh <- metadataFromContext(ctx).Copy():
	default:
	}
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

		inBody := []byte(nil)
		if req.GetPayload() != nil {
			inBody = req.GetPayload().GetBody()
		}

		if sendErr := stream.Send(&grpcTesting.StreamingOutputCallResponse{
			Payload: &grpcTesting.Payload{
				Body: slices.Concat([]byte(s.prefix), inBody),
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
	return runBackendTestServiceWithServer(t, &testBackendServer{prefix: prefix})
}

func runBackendTestServiceWithServer(t *testing.T, backend *testBackendServer) (string, func()) {
	t.Helper()

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	server := gogrpc.NewServer()
	grpcTesting.RegisterTestServiceServer(server, backend)
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

	const maxAttempts = 10
	for range maxAttempts {
		port := getFreeTCPPort(t)
		addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))

		server := gwGrpcServer.New(port)
		server.SetHandler(svc)
		server.Start()

		if waitTCPReady(addr) {
			return addr, func() {
				_ = server.Stop(3 * time.Second)
			}
		}

		_ = server.Stop(3 * time.Second)
	}

	require.FailNow(t, "failed to start gateway grpc server", "exhausted retry attempts")
	return "", func() {}
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

func waitTCPReady(addr string) bool {
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 50*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return false
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
				Backend: appModel.Backend{
					Url:     fmt.Sprintf("http://%s:%d", host, port),
					GrpcUrl: fmt.Sprintf("%s:%d", host, port),
				},
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
	snapshot.InheritDown()
	return snapshot
}

func buildSnapshotForBackendUnaryOnly(t *testing.T, backendAddr string) *rootModel.Root {
	t.Helper()

	snapshot := buildSnapshotForBackend(t, backendAddr)
	snapshot.Apps[0].Endpoints = snapshot.Apps[0].Endpoints[:1]
	require.NoError(t, snapshot.Normalize())
	snapshot.InheritDown()
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
				Backend: appModel.Backend{
					Url:     fmt.Sprintf("http://%s:%d", host1, port1),
					GrpcUrl: fmt.Sprintf("%s:%d", host1, port1),
				},
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
				Backend: appModel.Backend{
					Url:     fmt.Sprintf("http://%s:%d", host2, port2),
					GrpcUrl: fmt.Sprintf("%s:%d", host2, port2),
				},
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
	snapshot.InheritDown()
	return snapshot
}
