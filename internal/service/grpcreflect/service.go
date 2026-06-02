package grpcreflect

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	reflectionv1alpha "google.golang.org/grpc/reflection/grpc_reflection_v1alpha"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

type Endpoint struct {
	Service string
	Method  string
	Path    string
}

func LoadEndpoints(ctx context.Context, address string) ([]Endpoint, error) {
	address = strings.TrimSpace(address)
	if address == "" {
		return []Endpoint{}, nil
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("grpc.NewClient: %w", err)
	}
	defer func() { _ = conn.Close() }()

	client := reflectionv1alpha.NewServerReflectionClient(conn)
	services, err := listServices(ctx, client)
	if err != nil {
		return nil, err
	}

	result := make([]Endpoint, 0, len(services)*4)
	seen := make(map[string]struct{}, len(services)*4)
	for _, serviceName := range services {
		if isReflectionService(serviceName) {
			continue
		}
		files, loadErr := fileDescriptorsContainingSymbol(ctx, client, serviceName)
		if loadErr != nil {
			return nil, fmt.Errorf("load descriptor for %s: %w", serviceName, loadErr)
		}
		for _, file := range files {
			for _, svc := range file.GetService() {
				fullServiceName := joinProtoName(file.GetPackage(), svc.GetName())
				if fullServiceName != serviceName {
					continue
				}
				for _, method := range svc.GetMethod() {
					item := Endpoint{
						Service: fullServiceName,
						Method:  method.GetName(),
						Path:    "/" + fullServiceName + "/" + method.GetName(),
					}
					if _, ok := seen[item.Path]; ok {
						continue
					}
					seen[item.Path] = struct{}{}
					result = append(result, item)
				}
			}
		}
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].Service != result[j].Service {
			return result[i].Service < result[j].Service
		}
		return result[i].Method < result[j].Method
	})

	return result, nil
}

func SendSingleRequest(
	ctx context.Context,
	address string,
	req *reflectionv1alpha.ServerReflectionRequest,
) (*reflectionv1alpha.ServerReflectionResponse, error) {
	address = strings.TrimSpace(address)
	if address == "" {
		return nil, fmt.Errorf("address: empty")
	}
	if req == nil {
		return nil, fmt.Errorf("request: nil")
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("grpc.NewClient: %w", err)
	}
	defer func() { _ = conn.Close() }()

	stream, err := reflectionv1alpha.NewServerReflectionClient(conn).ServerReflectionInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("ServerReflectionInfo: %w", err)
	}
	if err = stream.Send(req); err != nil {
		return nil, fmt.Errorf("reflection send: %w", err)
	}
	if err = stream.CloseSend(); err != nil {
		return nil, fmt.Errorf("reflection close send: %w", err)
	}

	resp, err := stream.Recv()
	if err != nil {
		return nil, fmt.Errorf("reflection recv: %w", err)
	}
	return resp, nil
}

func listServices(ctx context.Context, client reflectionv1alpha.ServerReflectionClient) ([]string, error) {
	resp, err := sendClientRequest(ctx, client, &reflectionv1alpha.ServerReflectionRequest{
		MessageRequest: &reflectionv1alpha.ServerReflectionRequest_ListServices{
			ListServices: "",
		},
	})
	if err != nil {
		return nil, err
	}
	listResp := resp.GetListServicesResponse()
	if listResp == nil {
		return nil, fmt.Errorf("reflection list services: empty response")
	}

	services := make([]string, 0, len(listResp.GetService()))
	for _, svc := range listResp.GetService() {
		name := strings.TrimSpace(svc.GetName())
		if name != "" {
			services = append(services, name)
		}
	}
	sort.Strings(services)
	return services, nil
}

func fileDescriptorsContainingSymbol(
	ctx context.Context,
	client reflectionv1alpha.ServerReflectionClient,
	symbol string,
) ([]*descriptorpb.FileDescriptorProto, error) {
	resp, err := sendClientRequest(ctx, client, &reflectionv1alpha.ServerReflectionRequest{
		MessageRequest: &reflectionv1alpha.ServerReflectionRequest_FileContainingSymbol{
			FileContainingSymbol: symbol,
		},
	})
	if err != nil {
		return nil, err
	}
	fileResp := resp.GetFileDescriptorResponse()
	if fileResp == nil {
		return nil, fmt.Errorf("reflection file descriptor: empty response")
	}

	files := make([]*descriptorpb.FileDescriptorProto, 0, len(fileResp.GetFileDescriptorProto()))
	for _, raw := range fileResp.GetFileDescriptorProto() {
		file := &descriptorpb.FileDescriptorProto{}
		if unmarshalErr := proto.Unmarshal(raw, file); unmarshalErr != nil {
			return nil, fmt.Errorf("unmarshal file descriptor: %w", unmarshalErr)
		}
		files = append(files, file)
	}
	return files, nil
}

func sendClientRequest(
	ctx context.Context,
	client reflectionv1alpha.ServerReflectionClient,
	req *reflectionv1alpha.ServerReflectionRequest,
) (*reflectionv1alpha.ServerReflectionResponse, error) {
	stream, err := client.ServerReflectionInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("ServerReflectionInfo: %w", err)
	}
	if err = stream.Send(req); err != nil {
		return nil, fmt.Errorf("reflection send: %w", err)
	}
	if err = stream.CloseSend(); err != nil {
		return nil, fmt.Errorf("reflection close send: %w", err)
	}
	resp, err := stream.Recv()
	if err != nil {
		return nil, fmt.Errorf("reflection recv: %w", err)
	}
	if errorResp := resp.GetErrorResponse(); errorResp != nil {
		return nil, fmt.Errorf("reflection error %d: %s", errorResp.GetErrorCode(), errorResp.GetErrorMessage())
	}
	return resp, nil
}

func joinProtoName(pkg, name string) string {
	pkg = strings.TrimSpace(pkg)
	name = strings.TrimSpace(name)
	if pkg == "" {
		return name
	}
	return pkg + "." + name
}

func isReflectionService(serviceName string) bool {
	return serviceName == "grpc.reflection.v1alpha.ServerReflection" ||
		serviceName == "grpc.reflection.v1.ServerReflection"
}
