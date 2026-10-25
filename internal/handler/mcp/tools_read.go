package mcp

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/samber/lo"

	appModel "github.com/rendau/ruto/internal/domain/app/model"
	endpointModel "github.com/rendau/ruto/internal/domain/endpoint/model"
	"github.com/rendau/ruto/internal/domain/secretref"
	seenPathModel "github.com/rendau/ruto/internal/domain/seenpath/model"
	usecaseApp "github.com/rendau/ruto/internal/usecase/app"
	usecaseGateway "github.com/rendau/ruto/internal/usecase/gateway"
	usecaseMonitoring "github.com/rendau/ruto/internal/usecase/monitoring"
)

func (s *sessionServer) registerReadTools(srv *mcpsdk.Server) {
	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "get_root",
		Description: "Root configuration (gateway-wide CORS, JWT keys, auth, logging, variables) with secrets masked, plus its version for update_root.",
	}, s.getRoot)

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "list_apps",
		Description: "All apps: id, name, group, path_prefix, backend urls, allow_wildcard, active, endpoint count.",
	}, s.listApps)

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "get_app",
		Description: "Full app configuration with secrets masked, plus its version for update_app. app: id or exact name.",
	}, s.getApp)

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "list_endpoints",
		Description: "Endpoints of an app: id, type, method, path, custom_path, active, track_paths, allow_trailing_slash, whether transform scripts and server/browser cache are set. app: id or exact name.",
	}, s.listEndpoints)

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "get_endpoint",
		Description: "Full endpoint configuration (including transform scripts, cache, track_paths and allow_trailing_slash) with secrets masked, plus its version for update_endpoint.",
	}, s.getEndpoint)

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "get_endpoint_effective",
		Description: "The endpoint as the gateway applies it: auth, logging and variables inherited from root and app merged in. Secrets masked (only their length is shown).",
	}, s.getEndpointEffective)

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "get_swagger_diff",
		Description: "Compares the app's swagger with its endpoints: swagger operations without an endpoint and endpoints missing from swagger.",
	}, s.getSwaggerDiff)

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "list_seen_paths",
		Description: "Concrete paths the gateway served through wildcard endpoints of an app, with hit counts (ids collapsed to {id}), most requested first. Use it to replace a wildcard with exact endpoints. Only wildcard endpoints with track_paths=true are counted. endpoint_id narrows to one wildcard endpoint. Rows you turn into exact endpoints can be removed by delete_seen_paths in the same propose_changes request.",
	}, s.listSeenPaths)

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "get_publish_status",
		Description: "Published configuration version, the version a publish would produce now (they differ while there are unpublished changes) and the version each gateway runs.",
	}, s.getPublishStatus)

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "get_endpoint_metrics",
		Description: "Traffic of an endpoint from Prometheus: requests per second, error rate (5xx and failing gRPC codes) and average duration over a time range, as totals and a series.",
	}, s.getEndpointMetrics)

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "get_app_metrics",
		Description: "Traffic of a whole app from Prometheus, same shape as get_endpoint_metrics.",
	}, s.getAppMetrics)
}

// ── Objects ─────────────────────────────────────────────

type ObjectOut struct {
	Id      string `json:"id,omitempty"`
	Version string `json:"version"`
	Data    any    `json:"data"`
}

type EmptyIn struct{}

func (s *sessionServer) getRoot(ctx context.Context, req *mcpsdk.CallToolRequest, _ EmptyIn) (*mcpsdk.CallToolResult, ObjectOut, error) {
	return s.view(ctx, req, secretref.KindRoot, func(context.Context) (string, error) { return "", nil })
}

type AppIn struct {
	App string `json:"app" jsonschema:"app id or exact name"`
}

func (s *sessionServer) getApp(ctx context.Context, req *mcpsdk.CallToolRequest, in AppIn) (*mcpsdk.CallToolResult, ObjectOut, error) {
	return s.view(ctx, req, secretref.KindApp, func(ctx context.Context) (string, error) {
		app, err := s.resolveApp(ctx, in.App)
		if err != nil {
			return "", err
		}
		return app.Id, nil
	})
}

type IdIn struct {
	Id string `json:"id" jsonschema:"object id"`
}

func (s *sessionServer) getEndpoint(ctx context.Context, req *mcpsdk.CallToolRequest, in IdIn) (*mcpsdk.CallToolResult, ObjectOut, error) {
	return s.view(ctx, req, secretref.KindEndpoint, func(context.Context) (string, error) { return in.Id, nil })
}

func (s *sessionServer) view(
	ctx context.Context,
	req *mcpsdk.CallToolRequest,
	kind string,
	resolveId func(context.Context) (string, error),
) (*mcpsdk.CallToolResult, ObjectOut, error) {
	ctx, err := s.toolCtx(ctx, req)
	if err != nil {
		return nil, ObjectOut{}, err
	}

	id, err := resolveId(ctx)
	if err != nil {
		return nil, ObjectOut{}, toolErr(err)
	}

	item, err := s.h.changeRequestUc.View(ctx, kind, id)
	if err != nil {
		return nil, ObjectOut{}, toolErr(err)
	}

	return nil, ObjectOut{Id: item.Id, Version: item.Version, Data: item.Data}, nil
}

// resolveApp finds an app by id or by exact (case-insensitive) name.
func (s *sessionServer) resolveApp(ctx context.Context, ref string) (*appModel.App, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil, fmt.Errorf("app is required")
	}

	items, _, err := s.h.appUc.List(ctx, &appModel.ListReq{NameEqCI: &ref})
	if err != nil {
		return nil, err
	}
	if len(items) == 1 {
		return items[0], nil
	}

	items, _, err = s.h.appUc.List(ctx, &appModel.ListReq{IdsIn: &[]string{ref}})
	if err != nil {
		return nil, err
	}
	if len(items) == 1 {
		return items[0], nil
	}

	return nil, fmt.Errorf("app %q not found (by id or name)", ref)
}

// ── Apps and endpoints ──────────────────────────────────

type AppSummary struct {
	Id                 string `json:"id"`
	Version            string `json:"version"`
	Name               string `json:"name"`
	Group              string `json:"group,omitempty"`
	Active             bool   `json:"active"`
	PathPrefix         string `json:"path_prefix"`
	BackendUrl         string `json:"backend_url"`
	SwaggerUrl         string `json:"swagger_url,omitempty"`
	GrpcUrl            string `json:"grpc_url,omitempty"`
	AllowWildcard      bool   `json:"allow_wildcard"`
	ExcludeFromMetrics bool   `json:"exclude_from_metrics,omitempty"`
	EndpointsCount     int    `json:"endpoints_count"`
}

type AppListOut struct {
	Apps []AppSummary `json:"apps"`
}

func (s *sessionServer) listApps(ctx context.Context, req *mcpsdk.CallToolRequest, _ EmptyIn) (*mcpsdk.CallToolResult, AppListOut, error) {
	ctx, err := s.toolCtx(ctx, req)
	if err != nil {
		return nil, AppListOut{}, err
	}

	apps, _, err := s.h.appUc.List(ctx, &appModel.ListReq{})
	if err != nil {
		return nil, AppListOut{}, toolErr(err)
	}
	endpoints, _, err := s.h.endpointUc.List(ctx, &endpointModel.ListReq{})
	if err != nil {
		return nil, AppListOut{}, toolErr(err)
	}
	counts := lo.CountValuesBy(endpoints, func(ep *endpointModel.Endpoint) string { return ep.AppId })

	return nil, AppListOut{Apps: lo.Map(apps, func(v *appModel.App, _ int) AppSummary {
		return AppSummary{
			Id:                 v.Id,
			Version:            version(v),
			Name:               v.Name,
			Group:              v.Group,
			Active:             v.Active,
			PathPrefix:         v.PathPrefix,
			BackendUrl:         v.Backend.Url,
			SwaggerUrl:         v.Backend.SwaggerUrl,
			GrpcUrl:            v.Backend.GrpcUrl,
			AllowWildcard:      v.AllowWildcard,
			ExcludeFromMetrics: v.ExcludeFromMetrics,
			EndpointsCount:     counts[v.Id],
		}
	})}, nil
}

type EndpointSummary struct {
	Id                 string `json:"id"`
	Version            string `json:"version"`
	Active             bool   `json:"active"`
	Type               string `json:"type"`
	Method             string `json:"method,omitempty"`
	Path               string `json:"path"`
	CustomPath         string `json:"custom_path,omitempty"`
	TrackPaths         bool   `json:"track_paths,omitempty"`
	AllowTrailingSlash bool   `json:"allow_trailing_slash,omitempty"`
	TransformRequest   bool   `json:"transform_request,omitempty"`
	TransformResponse  bool   `json:"transform_response,omitempty"`
	ServerCache        bool   `json:"server_cache,omitempty"`
	BrowserCache       bool   `json:"browser_cache,omitempty"`
}

type EndpointListOut struct {
	AppId     string            `json:"app_id"`
	Endpoints []EndpointSummary `json:"endpoints"`
}

func (s *sessionServer) listEndpoints(ctx context.Context, req *mcpsdk.CallToolRequest, in AppIn) (*mcpsdk.CallToolResult, EndpointListOut, error) {
	ctx, err := s.toolCtx(ctx, req)
	if err != nil {
		return nil, EndpointListOut{}, err
	}

	app, err := s.resolveApp(ctx, in.App)
	if err != nil {
		return nil, EndpointListOut{}, toolErr(err)
	}

	items, _, err := s.h.endpointUc.List(ctx, &endpointModel.ListReq{AppId: &app.Id})
	if err != nil {
		return nil, EndpointListOut{}, toolErr(err)
	}

	return nil, EndpointListOut{AppId: app.Id, Endpoints: lo.Map(items, func(v *endpointModel.Endpoint, _ int) EndpointSummary {
		result := EndpointSummary{
			Id:                 v.Id,
			Version:            version(v),
			Active:             v.Active,
			Type:               string(v.Type),
			CustomPath:         v.Backend.CustomPath,
			TrackPaths:         v.TrackPaths,
			AllowTrailingSlash: v.AllowTrailingSlash,
			TransformRequest:   v.Transform.Request != "",
			TransformResponse:  v.Transform.Response != "",
			ServerCache:        v.Cache.Server.Enabled,
			BrowserCache:       v.Cache.Browser.Enabled,
		}
		if v.Type == endpointModel.TypeGRPC {
			result.Path = v.Grpc.Path
		} else {
			result.Method, result.Path = v.Http.Method, v.Http.Path
		}
		return result
	})}, nil
}

// version matches the one get_* returns, so a summary is enough for a delete.
func version(v any) string {
	result, _ := secretref.Version(v)
	return result
}

type DataOut struct {
	Data any `json:"data"`
}

func (s *sessionServer) getEndpointEffective(ctx context.Context, req *mcpsdk.CallToolRequest, in IdIn) (*mcpsdk.CallToolResult, DataOut, error) {
	ctx, err := s.toolCtx(ctx, req)
	if err != nil {
		return nil, DataOut{}, err
	}

	ep, err := s.h.endpointUc.Get(ctx, in.Id)
	if err != nil {
		return nil, DataOut{}, toolErr(err)
	}
	item, err := s.h.endpointUc.Inherited(ctx, in.Id, ep.Variables)
	if err != nil {
		return nil, DataOut{}, toolErr(err)
	}

	tree, err := secretref.ToTree(item)
	if err != nil {
		return nil, DataOut{}, err
	}

	return nil, DataOut{Data: secretref.MaskAnonymous(tree)}, nil
}

// ── Swagger, seen paths ─────────────────────────────────

type SwaggerOperation struct {
	Method string `json:"method"`
	Path   string `json:"path"`
}

type SwaggerDiffOut struct {
	// Unregistered: swagger operations without an endpoint.
	Unregistered []SwaggerOperation `json:"unregistered"`
	// RegisteredInvalid: endpoints whose operation is not in swagger.
	RegisteredInvalid []SwaggerOperation `json:"registered_invalid"`
}

func (s *sessionServer) getSwaggerDiff(ctx context.Context, req *mcpsdk.CallToolRequest, in AppIn) (*mcpsdk.CallToolResult, SwaggerDiffOut, error) {
	ctx, err := s.toolCtx(ctx, req)
	if err != nil {
		return nil, SwaggerDiffOut{}, err
	}

	app, err := s.resolveApp(ctx, in.App)
	if err != nil {
		return nil, SwaggerDiffOut{}, toolErr(err)
	}

	diff, err := s.h.appUc.GetSwaggerEndpointsDiff(ctx, app.Id)
	if err != nil {
		return nil, SwaggerDiffOut{}, toolErr(err)
	}

	toOut := func(v usecaseApp.SwaggerEndpoint, _ int) SwaggerOperation {
		return SwaggerOperation{Method: v.Method, Path: v.Path}
	}

	return nil, SwaggerDiffOut{
		Unregistered:      lo.Map(diff.Unregistered, toOut),
		RegisteredInvalid: lo.Map(diff.RegisteredInvalid, toOut),
	}, nil
}

type SeenPathsIn struct {
	App        string `json:"app" jsonschema:"app id or exact name"`
	EndpointId string `json:"endpoint_id,omitempty" jsonschema:"only paths served by this wildcard endpoint"`
}

type SeenPathOut struct {
	EndpointId   string `json:"endpoint_id"`
	Method       string `json:"method"`
	Path         string `json:"path"`
	Hits         int64  `json:"hits"`
	HitsNotFound int64  `json:"hits_not_found"`
	FirstSeenAt  string `json:"first_seen_at"`
	LastSeenAt   string `json:"last_seen_at"`
	Sample       string `json:"sample,omitempty"`
}

type SeenPathsOut struct {
	AppId string        `json:"app_id"`
	Paths []SeenPathOut `json:"paths"`
}

func (s *sessionServer) listSeenPaths(ctx context.Context, req *mcpsdk.CallToolRequest, in SeenPathsIn) (*mcpsdk.CallToolResult, SeenPathsOut, error) {
	ctx, err := s.toolCtx(ctx, req)
	if err != nil {
		return nil, SeenPathsOut{}, err
	}

	app, err := s.resolveApp(ctx, in.App)
	if err != nil {
		return nil, SeenPathsOut{}, toolErr(err)
	}

	items, err := s.h.seenPathUc.List(ctx, app.Id)
	if err != nil {
		return nil, SeenPathsOut{}, toolErr(err)
	}
	if in.EndpointId != "" {
		items = lo.Filter(items, func(v *seenPathModel.SeenPath, _ int) bool { return v.EndpointId == in.EndpointId })
	}
	slices.SortStableFunc(items, func(a, b *seenPathModel.SeenPath) int { return int(b.Hits - a.Hits) })

	return nil, SeenPathsOut{AppId: app.Id, Paths: lo.Map(items, func(v *seenPathModel.SeenPath, _ int) SeenPathOut {
		return SeenPathOut{
			EndpointId:   v.EndpointId,
			Method:       v.Method,
			Path:         v.Path,
			Hits:         v.Hits,
			HitsNotFound: v.HitsNotFound,
			FirstSeenAt:  v.FirstSeenAt.Format(time.RFC3339),
			LastSeenAt:   v.LastSeenAt.Format(time.RFC3339),
			Sample:       v.Sample,
		}
	})}, nil
}

// ── Publishing ──────────────────────────────────────────

type GatewayOut struct {
	GatewayId       string `json:"gateway_id"`
	HostName        string `json:"host_name"`
	Status          string `json:"status"`
	SnapshotVersion string `json:"snapshot_version"`
	UpToDate        bool   `json:"up_to_date"`
	LastApplyAt     string `json:"last_apply_at,omitempty"`
	LastError       string `json:"last_error,omitempty"`
}

type PublishStatusOut struct {
	PublishedVersion      string       `json:"published_version"`
	CurrentVersion        string       `json:"current_version"`
	HasUnpublishedChanges bool         `json:"has_unpublished_changes"`
	CurrentError          string       `json:"current_error,omitempty"`
	Gateways              []GatewayOut `json:"gateways"`
}

func (s *sessionServer) getPublishStatus(ctx context.Context, req *mcpsdk.CallToolRequest, _ EmptyIn) (*mcpsdk.CallToolResult, PublishStatusOut, error) {
	ctx, err := s.toolCtx(ctx, req)
	if err != nil {
		return nil, PublishStatusOut{}, err
	}

	result := PublishStatusOut{}

	if result.PublishedVersion, err = s.h.snapshotUc.GetVersion(ctx); err != nil {
		return nil, PublishStatusOut{}, toolErr(err)
	}
	if result.CurrentVersion, err = s.h.snapshotUc.CurrentVersion(ctx); err != nil {
		result.CurrentError = err.Error()
	}
	result.HasUnpublishedChanges = result.CurrentVersion != "" && result.CurrentVersion != result.PublishedVersion

	gateways, err := s.h.gatewayUc.List(ctx)
	if err != nil {
		return nil, PublishStatusOut{}, toolErr(err)
	}
	result.Gateways = lo.Map(gateways, func(v *usecaseGateway.Item, _ int) GatewayOut {
		out := GatewayOut{
			GatewayId:       v.GatewayID,
			HostName:        v.HostName,
			Status:          v.Status,
			SnapshotVersion: v.SnapshotVersion,
			UpToDate:        v.SnapshotVersion == result.PublishedVersion,
			LastError:       v.LastError,
		}
		if v.LastApplyAtUnix > 0 {
			out.LastApplyAt = time.Unix(v.LastApplyAtUnix, 0).Format(time.RFC3339)
		}
		return out
	})

	return nil, result, nil
}

// ── Metrics ─────────────────────────────────────────────

type MetricsIn struct {
	Id           string `json:"id" jsonschema:"endpoint id (get_endpoint_metrics) or app id (get_app_metrics)"`
	RangeMinutes int64  `json:"range_minutes,omitempty" jsonschema:"time range back from now in minutes: 5..43200, default 60"`
}

type MetricsPointOut struct {
	Ts                 string  `json:"ts"`
	Rps                float64 `json:"rps"`
	ErrorRate          float64 `json:"error_rate"`
	DurationAvgSeconds float64 `json:"duration_avg_seconds"`
}

type MetricsOut struct {
	RangeMinutes int64 `json:"range_minutes"`
	StepSeconds  int64 `json:"step_seconds"`
	// Requests and Errors are estimated totals over the range.
	Requests  float64           `json:"requests"`
	Errors    float64           `json:"errors"`
	ErrorRate float64           `json:"error_rate"`
	Points    []MetricsPointOut `json:"points"`
}

func (s *sessionServer) getEndpointMetrics(ctx context.Context, req *mcpsdk.CallToolRequest, in MetricsIn) (*mcpsdk.CallToolResult, MetricsOut, error) {
	return s.metrics(ctx, req, in, s.h.monitoringUc.EndpointMetrics)
}

func (s *sessionServer) getAppMetrics(ctx context.Context, req *mcpsdk.CallToolRequest, in MetricsIn) (*mcpsdk.CallToolResult, MetricsOut, error) {
	return s.metrics(ctx, req, in, s.h.monitoringUc.AppMetrics)
}

func (s *sessionServer) metrics(
	ctx context.Context,
	req *mcpsdk.CallToolRequest,
	in MetricsIn,
	query func(ctx context.Context, id string, rangeSeconds int64) (*usecaseMonitoring.Series, error),
) (*mcpsdk.CallToolResult, MetricsOut, error) {
	ctx, err := s.toolCtx(ctx, req)
	if err != nil {
		return nil, MetricsOut{}, err
	}

	if in.RangeMinutes <= 0 {
		in.RangeMinutes = 60
	}

	series, err := query(ctx, in.Id, in.RangeMinutes*60)
	if err != nil {
		return nil, MetricsOut{}, toolErr(err)
	}

	result := MetricsOut{RangeMinutes: in.RangeMinutes, StepSeconds: series.StepSeconds}
	for _, p := range series.Points {
		requests := p.Rps * float64(series.StepSeconds)
		result.Requests += requests
		result.Errors += requests * p.ErrorRate
		result.Points = append(result.Points, MetricsPointOut{
			Ts:                 time.Unix(p.Ts, 0).Format(time.RFC3339),
			Rps:                p.Rps,
			ErrorRate:          p.ErrorRate,
			DurationAvgSeconds: p.DurationAvgSeconds,
		})
	}
	if result.Requests > 0 {
		result.ErrorRate = result.Errors / result.Requests
	}

	return nil, result, nil
}
