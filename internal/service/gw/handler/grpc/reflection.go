package grpc

import (
	"sort"
	"strings"

	"google.golang.org/grpc/codes"
	reflectionv1alpha "google.golang.org/grpc/reflection/grpc_reflection_v1alpha"
)

type reflectionRoute struct {
	targetGrpcAddress string
	services          map[string]struct{}
	paths             map[string]struct{}
}

func (r *reflectionRoute) listServicesResponse() *reflectionv1alpha.ListServiceResponse {
	names := make([]string, 0, len(r.services))
	for serviceName := range r.services {
		names = append(names, serviceName)
	}
	sort.Strings(names)

	services := make([]*reflectionv1alpha.ServiceResponse, 0, len(names))
	for _, serviceName := range names {
		services = append(services, &reflectionv1alpha.ServiceResponse{
			Name: serviceName,
		})
	}

	return &reflectionv1alpha.ListServiceResponse{
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
	req *reflectionv1alpha.ServerReflectionRequest,
	code codes.Code,
	message string,
) *reflectionv1alpha.ServerReflectionResponse {
	validHost := ""
	if req != nil {
		validHost = req.GetHost()
	}
	return &reflectionv1alpha.ServerReflectionResponse{
		ValidHost:       validHost,
		OriginalRequest: req,
		MessageResponse: &reflectionv1alpha.ServerReflectionResponse_ErrorResponse{
			ErrorResponse: &reflectionv1alpha.ErrorResponse{
				ErrorCode:    int32(code),
				ErrorMessage: message,
			},
		},
	}
}
