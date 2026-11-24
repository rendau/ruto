# Monitoring (admin traffic charts & logs)

The admin SPA can show per-app and per-endpoint traffic charts (RPS, avg duration, error
rate) and recent access-log entries. ruto stores none of this itself — `core` queries an
external Prometheus / log store on demand and proxies the result to the SPA.

## Data flow

```
gateway ──/metrics──▶ Prometheus ◀──PromQL── core ◀──/api/monitoring/*── admin SPA
gateway ──stdout───▶ Loki / Graylog ◀──query── core
```

- **gateway** exports two metrics on `:SYSTEM_PORT/metrics` (enable with `WITH_METRICS=true`):
  - `<ns>_ruto_gw_http_requests_total{app, app_id, endpoint_id, protocol, method, status}`
  - `<ns>_ruto_gw_http_request_duration_seconds{app, app_id, endpoint_id, protocol, method}`

  `<ns>` is `METRICS_NAMESPACE` (default `company`). The `app_id` / `endpoint_id` labels are
  the correlation keys the admin uses; the same ids are stamped into every access-log line.
- **core** builds PromQL / LogQL / Lucene queries itself (the SPA never sends raw queries)
  and serves them under `/api/monitoring/*` (proto service `Monitoring`). Endpoint logs are
  restricted to users who can manage the app (they may contain payloads); metrics require
  only an authorized session.

## core configuration

All optional — with nothing set, `/monitoring/status` reports both features disabled and
the admin hides the UI.

```bash
# metrics
PROMETHEUS_URL=http://prometheus:9090
METRICS_NAMESPACE=company            # must match the gateway's METRICS_NAMESPACE

# logs — configure ONE provider (loki wins if both are set)
LOKI_URL=http://loki:3100
LOKI_SELECTOR={app="ruto-gateway"}   # stream selector for the gateway's stdout logs
LOKI_ORG_ID=                         # optional X-Scope-OrgID

GRAYLOG_URL=https://graylog.example.com
GRAYLOG_API_TOKEN=...                # access token (sent as basic auth token:token)
GRAYLOG_STREAM_ID=                   # optional stream filter
GRAYLOG_QUERY=                       # optional extra Lucene query AND-ed to every search
```

## Semantics

- Error rate counts server-side failures only: HTTP `5xx` and gRPC
  `Internal|Unknown|Unavailable|DeadlineExceeded|ResourceExhausted|DataLoss|Unimplemented`.
- Range is clamped to 5m…30d; core picks the step (~60 points per chart) and the rate window.
- Log search matches the `access log` marker plus the `endpoint_id` — for Graylog it works
  both with extracted fields and with the raw JSON line in `message`.
- Apps/endpoints with `exclude_from_metrics` produce no series; the admin hides their charts.

## Rate limit

The gateway exports (same `<ns>_ruto_` prefix):

- `gw_rate_limit_rejected_total{app, app_id, endpoint_id, protocol, scope, rule}` — requests
  rejected by a rate-limit rule (`rule` is the rule name, or `"<key> <requests>/<period>"`).
  Not exported for apps/endpoints with `exclude_from_metrics`.
- `gw_rate_limit_evicted_total` — counters dropped because the key table was full
  (`RATE_LIMIT_MAX_KEYS`); a steady rise means a flood of distinct clients or a cap too low.
- `gw_rate_limit_keys` — counters in memory, as of the last sweep (every 30s).

A rejected HTTP request is a `429` (not an error in the charts); a rejected gRPC call is
`ResourceExhausted`, which the error rate above counts as a failure.
