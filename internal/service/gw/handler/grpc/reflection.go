package grpc

import (
	"sort"
	"strings"

	"google.golang.org/grpc/codes"
	reflectionv1 "google.golang.org/grpc/reflection/grpc_reflection_v1"
)

type reflectionRoute struct {
	targetGrpcAddress string
	services          map[string]struct{}
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
	for serviceName := range r.services {
		if strings.HasPrefix(symbol, serviceName+".") {
			return true
		}
	}
	return false
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
