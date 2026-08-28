import { apiFetch } from "./http";
import { withQuery } from "./query";
import type {
  MonitoringEndpointsRps,
  MonitoringLogEntry,
  MonitoringSeries,
  MonitoringStatus
} from "./types";

// grpc-gateway serializes int64 as JSON strings — normalize to numbers here so
// the charts never do string arithmetic.

let statusPromise: Promise<MonitoringStatus> | null = null;

export function getMonitoringStatus(): Promise<MonitoringStatus> {
  if (!statusPromise) {
    statusPromise = apiFetch<MonitoringStatus>("/monitoring/status").catch((error) => {
      statusPromise = null;
      throw error;
    });
  }
  return statusPromise;
}

export async function getAppMetrics(id: string, rangeSeconds: number): Promise<MonitoringSeries> {
  return normalizeSeries(
    await apiFetch<MonitoringSeries>(
      withQuery(`/monitoring/app/${encodeURIComponent(id)}/metrics`, { range_seconds: rangeSeconds })
    )
  );
}

export async function getEndpointMetrics(
  id: string,
  rangeSeconds: number
): Promise<MonitoringSeries> {
  return normalizeSeries(
    await apiFetch<MonitoringSeries>(
      withQuery(`/monitoring/endpoint/${encodeURIComponent(id)}/metrics`, {
        range_seconds: rangeSeconds
      })
    )
  );
}

export async function getAppEndpointsRps(
  id: string,
  rangeSeconds: number
): Promise<MonitoringEndpointsRps> {
  const rep = await apiFetch<MonitoringEndpointsRps>(
    withQuery(`/monitoring/app/${encodeURIComponent(id)}/endpoints-rps`, {
      range_seconds: rangeSeconds
    })
  );
  return {
    step_seconds: Number(rep.step_seconds),
    results: (rep.results ?? []).map((item) => ({
      endpoint_id: item.endpoint_id,
      points: (item.points ?? []).map((p) => ({ ts: Number(p.ts), value: Number(p.value) }))
    }))
  };
}

export async function getEndpointLogs(
  id: string,
  options: { rangeSeconds: number; onlyErrors: boolean; limit?: number }
): Promise<MonitoringLogEntry[]> {
  const rep = await apiFetch<{ results: MonitoringLogEntry[] }>(
    withQuery(`/monitoring/endpoint/${encodeURIComponent(id)}/logs`, {
      range_seconds: options.rangeSeconds,
      only_errors: options.onlyErrors,
      limit: options.limit ?? 50
    })
  );
  return (rep.results ?? []).map((entry) => ({ ...entry, ts_ms: Number(entry.ts_ms) }));
}

function normalizeSeries(rep: MonitoringSeries): MonitoringSeries {
  return {
    step_seconds: Number(rep.step_seconds),
    points: (rep.points ?? []).map((p) => ({
      ts: Number(p.ts),
      rps: Number(p.rps),
      duration_avg_seconds: Number(p.duration_avg_seconds),
      error_rate: Number(p.error_rate)
    }))
  };
}
