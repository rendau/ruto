package dto

import (
	"github.com/samber/lo"

	usecase "github.com/rendau/ruto/internal/usecase/stats"
	"github.com/rendau/ruto/pkg/proto/ruto_v1"
)

func EncodeStatsResponse(v *usecase.Stats) *ruto_v1.StatsResponse {
	return &ruto_v1.StatsResponse{
		AppsTotal:         v.AppsTotal,
		AppsActive:        v.AppsActive,
		AppsInactive:      v.AppsInactive,
		EndpointsTotal:    v.EndpointsTotal,
		EndpointsActive:   v.EndpointsActive,
		EndpointsInactive: v.EndpointsInactive,
		UsersTotal:        v.UsersTotal,
		UsersActive:       v.UsersActive,
		UsersAdmin:        v.UsersAdmin,
		RootJwtProviders:  v.RootJWTProviders,
		RootAuthEnabled:   v.RootAuthEnabled,
		RootCorsEnabled:   v.RootCorsEnabled,
		Methods:           lo.Map(v.Methods, EncodeStatsMethodStats),
	}
}

func EncodeStatsMethodStats(v usecase.MethodStats, _ int) *ruto_v1.StatsMethodStats {
	return &ruto_v1.StatsMethodStats{
		Method: v.Method,
		Total:  v.Total,
		Active: v.Active,
	}
}
