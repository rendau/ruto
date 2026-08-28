package dto

import (
	"github.com/samber/lo"

	usecase "github.com/rendau/ruto/internal/usecase/monitoring"
	"github.com/rendau/ruto/pkg/proto/ruto_v1"
)

func EncodeMonitoringStatusResponse(v *usecase.Status) *ruto_v1.MonitoringStatusResponse {
	return &ruto_v1.MonitoringStatusResponse{
		MetricsEnabled: v.MetricsEnabled,
		LogsEnabled:    v.LogsEnabled,
		LogsProvider:   v.LogsProvider,
	}
}

func EncodeMonitoringSeriesResponse(v *usecase.Series) *ruto_v1.MonitoringSeriesResponse {
	return &ruto_v1.MonitoringSeriesResponse{
		StepSeconds: v.StepSeconds,
		Points:      lo.Map(v.Points, EncodeMonitoringSeriesPoint),
	}
}

func EncodeMonitoringSeriesPoint(v usecase.SeriesPoint, _ int) *ruto_v1.MonitoringSeriesPoint {
	return &ruto_v1.MonitoringSeriesPoint{
		Ts:                 v.Ts,
		Rps:                v.Rps,
		DurationAvgSeconds: v.DurationAvgSeconds,
		ErrorRate:          v.ErrorRate,
	}
}

func EncodeMonitoringEndpointsRpsResponse(v *usecase.EndpointsRps) *ruto_v1.MonitoringEndpointsRpsResponse {
	return &ruto_v1.MonitoringEndpointsRpsResponse{
		StepSeconds: v.StepSeconds,
		Results:     lo.Map(v.Results, EncodeMonitoringEndpointRps),
	}
}

func EncodeMonitoringEndpointRps(v usecase.EndpointRps, _ int) *ruto_v1.MonitoringEndpointRps {
	return &ruto_v1.MonitoringEndpointRps{
		EndpointId: v.EndpointId,
		Points:     lo.Map(v.Points, EncodeMonitoringValuePoint),
	}
}

func EncodeMonitoringValuePoint(v usecase.ValuePoint, _ int) *ruto_v1.MonitoringValuePoint {
	return &ruto_v1.MonitoringValuePoint{
		Ts:    v.Ts,
		Value: v.Value,
	}
}

func EncodeMonitoringLogEntry(v usecase.LogEntry, _ int) *ruto_v1.MonitoringLogEntry {
	return &ruto_v1.MonitoringLogEntry{
		TsMs:     v.TsMs,
		Status:   v.Status,
		Duration: v.Duration,
		Error:    v.Error,
		Message:  v.Message,
		Raw:      v.Raw,
	}
}
