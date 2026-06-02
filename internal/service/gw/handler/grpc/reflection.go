package grpc

import (
	"fmt"
	"sort"
	"strings"

	"google.golang.org/grpc/codes"
	reflectionv1 "google.golang.org/grpc/reflection/grpc_reflection_v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

type reflectionRoute struct {
	targetGrpcAddress string
	services          map[string]struct{}
	methods           map[string]map[string]struct{}
	paths             map[string]struct{}
}

func (r *reflectionRoute) listServicesResponse() *reflectionv1.ListServiceResponse {
	names := make([]string, 0, len(r.services))
	for serviceName := range r.services {
		names = append(names, serviceName)
	}
	sort.Strings(names)

	services := make([]*reflectionv1.ServiceResponse, 0, len(names))
	for _, serviceName := range names {
		services = append(services, &reflectionv1.ServiceResponse{
			Name: serviceName,
		})
	}

	return &reflectionv1.ListServiceResponse{
		Service: services,
	}
}

func (r *reflectionRoute) isAllowedSymbol(symbol string) bool {
	symbol = strings.TrimSpace(symbol)
	if symbol == "" {
		return false
	}
	if _, ok := r.services[symbol]; ok {
		return true
	}
	if _, ok := r.paths[symbol]; ok {
		return true
	}
	for serviceName, methods := range r.methods {
		methodName, ok := strings.CutPrefix(symbol, serviceName+".")
		if !ok {
			continue
		}
		if _, ok = methods[methodName]; ok {
			return true
		}
	}
	return false
}

func (r *reflectionRoute) filterFileDescriptorResponse(fileResp *reflectionv1.FileDescriptorResponse) error {
	if fileResp == nil {
		return nil
	}

	filteredRaw := make([][]byte, 0, len(fileResp.GetFileDescriptorProto()))
	for _, raw := range fileResp.GetFileDescriptorProto() {
		file := &descriptorpb.FileDescriptorProto{}
		if err := proto.Unmarshal(raw, file); err != nil {
			return fmt.Errorf("unmarshal file descriptor: %w", err)
		}

		services := file.GetService()
		if len(services) > 0 {
			filteredServices := make([]*descriptorpb.ServiceDescriptorProto, 0, len(services))
			for _, svc := range services {
				if svc == nil {
					continue
				}

				fullServiceName := joinProtoName(file.GetPackage(), svc.GetName())
				allowedMethods, ok := r.methods[fullServiceName]
				if !ok {
					continue
				}

				filteredMethods := make([]*descriptorpb.MethodDescriptorProto, 0, len(svc.GetMethod()))
				for _, method := range svc.GetMethod() {
					if method == nil {
						continue
					}
					if _, ok = allowedMethods[method.GetName()]; ok {
						filteredMethods = append(filteredMethods, method)
					}
				}
				if len(filteredMethods) == 0 {
					continue
				}

				svc.Method = filteredMethods
				filteredServices = append(filteredServices, svc)
			}
			if len(filteredServices) == 0 {
				continue
			}
			file.Service = filteredServices
		}

		filtered, err := proto.Marshal(file)
		if err != nil {
			return fmt.Errorf("marshal file descriptor: %w", err)
		}
		filteredRaw = append(filteredRaw, filtered)
	}

	fileResp.FileDescriptorProto = filteredRaw
	return nil
}

func reflectionErrorResponse(
	req *reflectionv1.ServerReflectionRequest,
	code codes.Code,
	message string,
) *reflectionv1.ServerReflectionResponse {
	validHost := ""
	if req != nil {
		validHost = req.GetHost()
	}
	return &reflectionv1.ServerReflectionResponse{
		ValidHost:       validHost,
		OriginalRequest: req,
		MessageResponse: &reflectionv1.ServerReflectionResponse_ErrorResponse{
			ErrorResponse: &reflectionv1.ErrorResponse{
				ErrorCode:    int32(code),
				ErrorMessage: message,
			},
		},
	}
}

func joinProtoName(pkg, name string) string {
	pkg = strings.TrimSpace(pkg)
	name = strings.TrimSpace(name)
	if pkg == "" {
		return name
	}
	return pkg + "." + name
}
