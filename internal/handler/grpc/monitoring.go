package grpc

import (
	"context"

	"github.com/samber/lo"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/rendau/ruto/internal/handler/grpc/dto"
	usecase "github.com/rendau/ruto/internal/usecase/monitoring"
	"github.com/rendau/ruto/pkg/proto/ruto_v1"
)

type Monitoring struct {
	ruto_v1.UnsafeMonitoringServer
	usecase *usecase.Usecase
}

func NewMonitoring(usecase *usecase.Usecase) *Monitoring {
	return &Monitoring{usecase: usecase}
}

func (h *Monitoring) GetStatus(ctx context.Context, _ *emptypb.Empty) (*ruto_v1.MonitoringStatusResponse, error) {
	result, err := h.usecase.GetStatus(ctx)
	if err != nil {
		return nil, err
	}
	return dto.EncodeMonitoringStatusResponse(result), nil
}

func (h *Monitoring) AppMetrics(ctx context.Context, req *ruto_v1.MonitoringMetricsRequest) (*ruto_v1.MonitoringSeriesResponse, error) {
	result, err := h.usecase.AppMetrics(ctx, req.GetId(), req.GetRangeSeconds())
	if err != nil {
		return nil, err
	}
	return dto.EncodeMonitoringSeriesResponse(result), nil
}

func (h *Monitoring) EndpointMetrics(ctx context.Context, req *ruto_v1.MonitoringMetricsRequest) (*ruto_v1.MonitoringSeriesResponse, error) {
	result, err := h.usecase.EndpointMetrics(ctx, req.GetId(), req.GetRangeSeconds())
	if err != nil {
		return nil, err
	}
	return dto.EncodeMonitoringSeriesResponse(result), nil
}

func (h *Monitoring) AppEndpointsRps(ctx context.Context, req *ruto_v1.MonitoringMetricsRequest) (*ruto_v1.MonitoringEndpointsRpsResponse, error) {
	result, err := h.usecase.AppEndpointsRps(ctx, req.GetId(), req.GetRangeSeconds())
	if err != nil {
		return nil, err
	}
	return dto.EncodeMonitoringEndpointsRpsResponse(result), nil
}

func (h *Monitoring) EndpointLogs(ctx context.Context, req *ruto_v1.MonitoringLogsRequest) (*ruto_v1.MonitoringLogsResponse, error) {
	result, err := h.usecase.EndpointLogs(ctx, req.GetId(), req.GetRangeSeconds(), req.GetOnlyErrors(), req.GetLimit())
	if err != nil {
		return nil, err
	}
	return &ruto_v1.MonitoringLogsResponse{
		Results: lo.Map(result, dto.EncodeMonitoringLogEntry),
	}, nil
}
