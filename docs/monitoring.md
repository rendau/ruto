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

## Client stats

The admin "Security" page shows **who** calls the gateway. Unlike the charts above it does
not need Prometheus: core stores these counters itself, in PostgreSQL.

- **Source.** Every gateway sends per-client counters in each heartbeat (every 30 s):
  `GatewayHeartbeatRequest.client_stats` — `{ip, trusted, requests, blocked, limited,
  last_rule}` since the previous heartbeat. `ip` is the client address (an IPv6 client as its
  `/64`), the network of a trusted client (`trusted` is then its name), or `""` for the clients
  that did not fit the bounded report. `blocked` — refused by the block list, `limited` —
  rejected by a rate-limit rule, `last_rule` — the rule of the latest rejection.
- **Storage.** Table `client_stat`, one row per 5-minute bucket and `ip` (`PRIMARY KEY
  (bucket, ip)`). The bucket is the time core *received* the heartbeat, truncated to 5 minutes
  (UTC). Reports of all gateways add up into the same row; `last_rule` keeps the newest
  non-empty one. A failed write fails the heartbeat, so the gateway keeps its counters and
  resends them, as with seen paths/origins.
- **Retention.** 7 days: every core replica deletes older buckets once an hour (several
  replicas doing it at once is harmless).
- **Admin API** (proto service `ClientStat`, `/api/client_stat/*`, administrators only, api
  keys are not accepted):
  - `overview?range_seconds=` — timeline of all traffic (requests, blocked, limited, trusted),
    totals with the number of distinct clients, and the traffic of each trusted client;
  - `top?kind=requests|blocked|limited|trusted&limit=&range_seconds=` — clients of the range,
    busiest first, with the active block covering each of them;
  - `series?ip=&range_seconds=` — timeline of one client (`ip` as `top` returns it).

  Range is clamped to 5m…7d (default 24h), `limit` to 1…200 (default 50). The step is the
  smallest multiple of 5 minutes giving at most ~100 points (24h → 15m, 7d → 105m); steps
  start on multiples of the step in unix time and empty steps are returned as zeros.
