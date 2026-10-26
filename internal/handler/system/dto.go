package system

import (
	"time"

	"github.com/samber/lo"

	"github.com/rendau/ruto/internal/infra/pulsekit"
	usecaseGateway "github.com/rendau/ruto/internal/usecase/gateway"
	usecaseSystem "github.com/rendau/ruto/internal/usecase/system"
)

// shortVersion — the snapshot hash is 64 hex chars; 12 identify a version as
// well and read easier.
const shortVersion = 12

// RoutesVersionRep and RoutesRep keep the shape pulse used to read from the
// snapshot ({"version"}, {"data": {"base_url", "apps"}}).
type RoutesVersionRep struct {
	Version string `json:"version"`
}

type RoutesRep struct {
	Version string        `json:"version"`
	Data    RoutesDataRep `json:"data"`
}

type RoutesDataRep struct {
	BaseUrl string         `json:"base_url"`
	Apps    []*RouteAppRep `json:"apps"`
}

type RouteAppRep struct {
	Id                 string              `json:"id"`
	Name               string              `json:"name"`
	Active             bool                `json:"active"`
	PathPrefix         string              `json:"path_prefix"`
	ExcludeFromMetrics bool                `json:"exclude_from_metrics"`
	Backend            RouteBackendRep     `json:"backend"`
	Endpoints          []*RouteEndpointRep `json:"endpoints"`
}

type RouteBackendRep struct {
	Url     string `json:"url"`
	GrpcUrl string `json:"grpc_url"`
}

type RouteEndpointRep struct {
	Id     string           `json:"id"`
	Active bool             `json:"active"`
	Type   string           `json:"type"`
	Http   RouteHttpRep     `json:"http"`
	Grpc   RouteGrpcPathRep `json:"grpc"`
}

type RouteHttpRep struct {
	Method string `json:"method"`
	Path   string `json:"path"`
}

type RouteGrpcPathRep struct {
	Path string `json:"path"`
}

func EncodeRoutes(v *usecaseSystem.Routes) *RoutesRep {
	return &RoutesRep{
		Version: v.Version,
		Data: RoutesDataRep{
			BaseUrl: v.BaseUrl,
			Apps:    lo.Map(v.Apps, EncodeRouteApp),
		},
	}
}

func EncodeRouteApp(v *usecaseSystem.RouteApp, _ int) *RouteAppRep {
	return &RouteAppRep{
		Id:                 v.Id,
		Name:               v.Name,
		Active:             v.Active,
		PathPrefix:         v.PathPrefix,
		ExcludeFromMetrics: v.ExcludeFromMetrics,
		Backend:            RouteBackendRep{Url: v.BackendUrl, GrpcUrl: v.GrpcUrl},
		Endpoints:          lo.Map(v.Endpoints, EncodeRouteEndpoint),
	}
}

func EncodeRouteEndpoint(v *usecaseSystem.RouteEndpoint, _ int) *RouteEndpointRep {
	return &RouteEndpointRep{
		Id:     v.Id,
		Active: v.Active,
		Type:   v.Type,
		Http:   RouteHttpRep{Method: v.Method, Path: v.Path},
		Grpc:   RouteGrpcPathRep{Path: v.GrpcPath},
	}
}

// GatewaysRep — the response of the pulse endpoint "gateways"; its schema is
// built from this type, so only these fields ever reach pulse.
type GatewaysRep struct {
	PublishedVersion      string        `json:"published_version" pulse:"description=опубликованная версия конфигурации (первые 12 символов)"`
	HasUnpublishedChanges bool          `json:"has_unpublished_changes" pulse:"description=есть сохранённые, но не опубликованные правки"`
	Gateways              []*GatewayRep `json:"gateways" pulse:"maxItems=50"`
}

type GatewayStatus string

func (GatewayStatus) PulseEnum() []string {
	return []string{usecaseGateway.StatusOnline, usecaseGateway.StatusStale, usecaseGateway.StatusOffline}
}

type GatewayRep struct {
	HostName    string        `json:"host_name" pulse:"maxLength=100,description=имя пода шлюза"`
	Status      GatewayStatus `json:"status" pulse:"description=online — отчитался за последние 70 с, stale — до 3 мин, offline — дольше"`
	Connected   bool          `json:"connected" pulse:"description=держит канал уведомлений с core"`
	Silent      bool          `json:"silent" pulse:"description=на связи, но не отчитывается дольше 3 мин — проблема"`
	Version     string        `json:"version" pulse:"maxLength=12,description=версия конфигурации шлюза (первые 12 символов)"`
	UpToDate    bool          `json:"up_to_date" pulse:"description=на опубликованной версии"`
	Lagging     bool          `json:"lagging" pulse:"description=на связи, но на старой версии дольше 2 мин после публикации — проблема"`
	LastApplyAt *time.Time    `json:"last_apply_at" pulse:"description=когда шлюз последний раз применил конфигурацию"`
	LastSeenAt  *time.Time    `json:"last_seen_at" pulse:"description=последний отчёт шлюза"`
	Problem     string        `json:"problem" pulse:"maxLength=100,description=последний сбой шлюза своими словами; пусто — сбоя нет"`
}

func EncodeGateways(v *usecaseSystem.Gateways, hasUnpublishedChanges bool) *GatewaysRep {
	return &GatewaysRep{
		PublishedVersion:      short(v.PublishedVersion),
		HasUnpublishedChanges: hasUnpublishedChanges,
		Gateways:              lo.Map(v.Gateways, EncodeGateway),
	}
}

func EncodeGateway(v *usecaseSystem.Gateway, _ int) *GatewayRep {
	return &GatewayRep{
		HostName:    v.HostName,
		Status:      GatewayStatus(v.Status),
		Connected:   v.Connected,
		Silent:      v.Silent,
		Version:     short(v.Version),
		UpToDate:    v.UpToDate,
		Lagging:     v.Lagging,
		LastApplyAt: v.LastApplyAt,
		LastSeenAt:  v.LastSeenAt,
		// the gateway reports the raw error text: it may hold addresses
		Problem: pulsekit.DescribeText(v.LastError),
	}
}

func short(version string) string {
	if len(version) > shortVersion {
		return version[:shortVersion]
	}
	return version
}
