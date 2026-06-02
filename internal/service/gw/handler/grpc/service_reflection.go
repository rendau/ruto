package grpc

import (
	"context"
	"io"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	reflectionv1alpha "google.golang.org/grpc/reflection/grpc_reflection_v1alpha"
	"google.golang.org/grpc/status"

	domEndpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
	domRootModel "github.com/rendau/ruto/internal/domain/root/model"
	"github.com/rendau/ruto/internal/service/grpcreflect"
)

func (s *Service) buildReflectionRoutes(snapshot *domRootModel.Root) map[string]*reflectionRoute {
	routes := make(map[string]*reflectionRoute)

	for _, app := range snapshot.ActiveApps() {
		targetGrpcAddress := app.GrpcAddress()
		if targetGrpcAddress == "" {
			continue
		}

		services := make(map[string]struct{})
		paths := make(map[string]struct{})
		for _, ep := range app.ActiveEndpoints() {
			if ep.Type != domEndpointModel.TypeGRPC {
				continue
			}
			services[ep.Grpc.Service] = struct{}{}
			paths[ep.Grpc.Path] = struct{}{}
		}
		if len(services) == 0 {
			continue
		}

		routes[strings.ToLower(app.Name)] = &reflectionRoute{
			targetGrpcAddress: targetGrpcAddress,
			services:          services,
			paths:             paths,
		}
	}

	return routes
}

func (s *Service) ServerReflectionInfo(stream reflectionv1alpha.ServerReflection_ServerReflectionInfoServer) error {
	md := metadataFromContext(stream.Context())
	rt := s.resolveReflectionRoute(md)
	if rt == nil {
		return status.Error(codes.NotFound, "route not found")
	}

	for {
		req, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		resp := s.handleReflectionRequest(stream.Context(), rt, req)
		if err = stream.Send(resp); err != nil {
			return err
		}
	}
}

func (s *Service) handleReflectionRequest(
	ctx context.Context,
	rt *reflectionRoute,
	req *reflectionv1alpha.ServerReflectionRequest,
) *reflectionv1alpha.ServerReflectionResponse {
	if req == nil {
		return reflectionErrorResponse(req, codes.InvalidArgument, "request is nil")
	}

	if _, ok := req.GetMessageRequest().(*reflectionv1alpha.ServerReflectionRequest_ListServices); ok {
		return &reflectionv1alpha.ServerReflectionResponse{
			ValidHost:       req.GetHost(),
			OriginalRequest: req,
			MessageResponse: &reflectionv1alpha.ServerReflectionResponse_ListServicesResponse{
				ListServicesResponse: rt.listServicesResponse(),
			},
		}
	}

	if symbol := req.GetFileContainingSymbol(); symbol != "" && !rt.isAllowedSymbol(symbol) {
		return reflectionErrorResponse(req, codes.NotFound, "symbol not registered")
	}

	resp, err := grpcreflect.SendSingleRequest(ctx, rt.targetGrpcAddress, req)
	if err != nil {
		return reflectionErrorResponse(req, codes.Unavailable, err.Error())
	}
	return resp
}

func (s *Service) resolveReflectionRoute(md metadata.MD) *reflectionRoute {
	appName := strings.TrimSpace(metadataFirstValue(md, metadataHeaderAppName))
	if appName == "" {
		return nil
	}

	rt, ok := s.reflectionRoutes[strings.ToLower(appName)]
	if !ok {
		return nil
	}

	return rt
}
