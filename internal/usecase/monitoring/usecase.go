package monitoring

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/samber/lo"
	"golang.org/x/sync/errgroup"

	"github.com/rendau/ruto/internal/errs"
	logstoreModel "github.com/rendau/ruto/internal/service/logstore/model"
	prometheusModel "github.com/rendau/ruto/internal/service/prometheus/model"
)

const (
	rangeSecondsDefault = 3600
	rangeSecondsMin     = 300
	rangeSecondsMax     = 30 * 24 * 3600

	// targetPoints is the desired number of points per chart; the actual step is
	// rounded up to a multiple of stepQuantum.
	targetPoints = 60
	stepQuantum  = 15

	logsLimitDefault = 50
	logsLimitMax     = 500

	// errorStatusRegex matches server-side failures only: HTTP 5xx and gRPC
	// codes that indicate a server/upstream problem (client errors like
	// NotFound/InvalidArgument are excluded, same as HTTP 4xx).
	errorStatusRegex = `5..|Internal|Unknown|Unavailable|DeadlineExceeded|ResourceExhausted|DataLoss|Unimplemented`
)

type Usecase struct {
	promSvc       PrometheusServiceI
	logSvc        LogStoreServiceI
	endpointSvc   EndpointServiceI
	sessionSvc    SessionServiceI
	metricsPrefix string // fully-qualified metric name prefix, e.g. "company_ruto_"
	logsProvider  string
}

func New(
	promSvc PrometheusServiceI,
	logSvc LogStoreServiceI,
	endpointSvc EndpointServiceI,
	sessionSvc SessionServiceI,
	metricsPrefix string,
	logsProvider string,
) *Usecase {
	return &Usecase{
		promSvc:       promSvc,
		logSvc:        logSvc,
		endpointSvc:   endpointSvc,
		sessionSvc:    sessionSvc,
		metricsPrefix: metricsPrefix,
		logsProvider:  logsProvider,
	}
}

func (u *Usecase) GetStatus(ctx context.Context) (*Status, error) {
	if !u.sessionSvc.CtxIsAuthorized(ctx) {
		return nil, errs.NotAuthorized
	}

	return &Status{
		MetricsEnabled: u.promSvc != nil,
		LogsEnabled:    u.logSvc != nil,
		LogsProvider:   u.logsProvider,
	}, nil
}

func (u *Usecase) AppMetrics(ctx context.Context, id string, rangeSeconds int64) (*Series, error) {
	return u.metrics(ctx, "app_id", id, rangeSeconds)
}

func (u *Usecase) EndpointMetrics(ctx context.Context, id string, rangeSeconds int64) (*Series, error) {
	return u.metrics(ctx, "endpoint_id", id, rangeSeconds)
}

func (u *Usecase) metrics(ctx context.Context, labelName, id string, rangeSeconds int64) (*Series, error) {
	if !u.sessionSvc.CtxIsAuthorized(ctx) {
		return nil, errs.NotAuthorized
	}
	if u.promSvc == nil {
		return nil, errs.MetricsNotConfigured
	}
	if id == "" {
		return nil, errs.IdRequired
	}

	start, end, step, rateWindow := rangePars(rangeSeconds)

	selector := labelName + "=" + strconv.Quote(id)
	counter := u.metricsPrefix + "gw_http_requests_total"
	histogram := u.metricsPrefix + "gw_http_request_duration_seconds"

	rpsQuery := `sum(rate(` + counter + `{` + selector + `}[` + rateWindow + `]))`
	errQuery := `sum(rate(` + counter + `{` + selector + `,status=~"` + errorStatusRegex + `"}[` + rateWindow + `]))`
	durQuery := `sum(rate(` + histogram + `_sum{` + selector + `}[` + rateWindow + `]))` +
		` / sum(rate(` + histogram + `_count{` + selector + `}[` + rateWindow + `]))`

	var rpsSeries, errSeries, durSeries []prometheusModel.Series

	eg, egCtx := errgroup.WithContext(ctx)
	eg.Go(func() (err error) {
		rpsSeries, err = u.promSvc.QueryRange(egCtx, rpsQuery, start, end, step)
		return err
	})
	eg.Go(func() (err error) {
		errSeries, err = u.promSvc.QueryRange(egCtx, errQuery, start, end, step)
		return err
	})
	eg.Go(func() (err error) {
		durSeries, err = u.promSvc.QueryRange(egCtx, durQuery, start, end, step)
		return err
	})
	if err := eg.Wait(); err != nil {
		return nil, fmt.Errorf("promSvc.QueryRange: %w", err)
	}

	return mergeSeries(rpsSeries, errSeries, durSeries, step), nil
}

func (u *Usecase) AppEndpointsRps(ctx context.Context, id string, rangeSeconds int64) (*EndpointsRps, error) {
	if !u.sessionSvc.CtxIsAuthorized(ctx) {
		return nil, errs.NotAuthorized
	}
	if u.promSvc == nil {
		return nil, errs.MetricsNotConfigured
	}
	if id == "" {
		return nil, errs.IdRequired
	}

	start, end, step, rateWindow := rangePars(rangeSeconds)

	query := `sum by (endpoint_id) (rate(` + u.metricsPrefix + `gw_http_requests_total{app_id=` +
		strconv.Quote(id) + `}[` + rateWindow + `]))`

	series, err := u.promSvc.QueryRange(ctx, query, start, end, step)
	if err != nil {
		return nil, fmt.Errorf("promSvc.QueryRange: %w", err)
	}

	return &EndpointsRps{
		StepSeconds: int64(step.Seconds()),
		Results: lo.FilterMap(series, func(v prometheusModel.Series, _ int) (EndpointRps, bool) {
			endpointId := v.Labels["endpoint_id"]
			if endpointId == "" {
				return EndpointRps{}, false
			}
			return EndpointRps{
				EndpointId: endpointId,
				Points:     lo.Map(v.Points, decodeValuePoint),
			}, true
		}),
	}, nil
}

func (u *Usecase) EndpointLogs(
	ctx context.Context,
	id string,
	rangeSeconds int64,
	onlyErrors bool,
	limit int64,
) ([]LogEntry, error) {
	if !u.sessionSvc.CtxIsAuthorized(ctx) {
		return nil, errs.NotAuthorized
	}
	if u.logSvc == nil {
		return nil, errs.LogsNotConfigured
	}
	if id == "" {
		return nil, errs.IdRequired
	}

	endpoint, _, err := u.endpointSvc.Get(ctx, id, true)
	if err != nil {
		return nil, fmt.Errorf("endpointSvc.Get: %w", err)
	}

	// Logs may carry request/response payloads, so unlike metrics they are
	// restricted to users who can manage the app.
	if !u.canManageApp(ctx, endpoint.AppId) {
		return nil, errs.NoPermission
	}

	if limit <= 0 {
		limit = logsLimitDefault
	}
	limit = min(limit, logsLimitMax)

	rangeSeconds = clampRangeSeconds(rangeSeconds)
	now := time.Now()

	entries, err := u.logSvc.List(ctx, &logstoreModel.ListReq{
		EndpointId: id,
		Since:      now.Add(-time.Duration(rangeSeconds) * time.Second),
		Until:      now,
		OnlyErrors: onlyErrors,
		Limit:      int(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("logSvc.List: %w", err)
	}

	return lo.Map(entries, decodeLogEntry), nil
}

func (u *Usecase) canManageApp(ctx context.Context, appId string) bool {
	if u.sessionSvc.CtxHasFullAppAccess(ctx) {
		return true
	}
	return lo.Contains(u.sessionSvc.CtxGetAppIds(ctx), appId)
}

func clampRangeSeconds(rangeSeconds int64) int64 {
	if rangeSeconds <= 0 {
		return rangeSecondsDefault
	}
	return min(max(rangeSeconds, rangeSecondsMin), rangeSecondsMax)
}

func rangePars(rangeSeconds int64) (start, end time.Time, step time.Duration, rateWindow string) {
	rangeSeconds = clampRangeSeconds(rangeSeconds)

	stepSeconds := (rangeSeconds/targetPoints + stepQuantum - 1) / stepQuantum * stepQuantum
	stepSeconds = max(stepSeconds, stepQuantum)

	rateWindowSeconds := max(4*stepSeconds, 60)

	end = time.Now().Truncate(time.Duration(stepSeconds) * time.Second)
	start = end.Add(-time.Duration(rangeSeconds) * time.Second)

	return start, end, time.Duration(stepSeconds) * time.Second, strconv.FormatInt(rateWindowSeconds, 10) + "s"
}

func mergeSeries(rpsSeries, errSeries, durSeries []prometheusModel.Series, step time.Duration) *Series {
	pointMap := map[int64]*SeriesPoint{}

	point := func(ts int64) *SeriesPoint {
		p, ok := pointMap[ts]
		if !ok {
			p = &SeriesPoint{Ts: ts}
			pointMap[ts] = p
		}
		return p
	}

	for _, s := range rpsSeries {
		for _, v := range s.Points {
			point(v.Ts).Rps = v.Value
		}
	}
	for _, s := range durSeries {
		for _, v := range s.Points {
			point(v.Ts).DurationAvgSeconds = v.Value
		}
	}
	for _, s := range errSeries {
		for _, v := range s.Points {
			if p := point(v.Ts); p.Rps > 0 {
				p.ErrorRate = min(v.Value/p.Rps, 1)
			}
		}
	}

	points := make([]SeriesPoint, 0, len(pointMap))
	for _, p := range pointMap {
		points = append(points, *p)
	}
	sort.Slice(points, func(i, j int) bool { return points[i].Ts < points[j].Ts })

	return &Series{
		StepSeconds: int64(step.Seconds()),
		Points:      points,
	}
}

func decodeValuePoint(v prometheusModel.Point, _ int) ValuePoint {
	return ValuePoint{Ts: v.Ts, Value: v.Value}
}

func decodeLogEntry(v *logstoreModel.Entry, _ int) LogEntry {
	return LogEntry{
		TsMs:     v.Ts.UnixMilli(),
		Status:   v.Status,
		Duration: v.Duration,
		Error:    v.Error,
		Message:  v.Message,
		Raw:      v.Raw,
	}
}
