import { apiFetch } from "./http";
import { withQuery } from "./query";
import type {
  ClientStatBlock,
  ClientStatIpLimit,
  ClientStatItem,
  ClientStatKind,
  ClientStatOverviewRep,
  ClientStatPoint,
  ClientStatSeriesRep,
  ClientStatTopRep,
  ClientStatTotals,
  ClientStatTrusted
} from "./types";

// grpc-gateway sends int64 as JSON strings; a message that isn't set comes as
// null. Everything is turned into finite numbers / plain values here.
type Raw<T> = { [K in keyof T]?: unknown };

function count(value: unknown): number {
  const n = Math.trunc(Number(value));
  return Number.isFinite(n) && n > 0 ? n : 0;
}

function rate(value: unknown): number {
  const n = Number(value);
  return Number.isFinite(n) && n > 0 ? n : 0;
}

function text(value: unknown): string {
  return typeof value === "string" ? value : "";
}

function list<T>(value: unknown): Raw<T>[] {
  return Array.isArray(value) ? (value.filter(Boolean) as Raw<T>[]) : [];
}

function normalizePoint(value: Raw<ClientStatPoint>): ClientStatPoint {
  return {
    ts: count(value.ts),
    requests: count(value.requests),
    blocked: count(value.blocked),
    limited: count(value.limited),
    trusted: count(value.trusted)
  };
}

function normalizePoints(value: unknown): ClientStatPoint[] {
  return list<ClientStatPoint>(value)
    .map(normalizePoint)
    .sort((a, b) => a.ts - b.ts);
}

function normalizeTotals(value: Raw<ClientStatTotals> | null | undefined): ClientStatTotals {
  return {
    requests: count(value?.requests),
    blocked: count(value?.blocked),
    limited: count(value?.limited),
    trusted: count(value?.trusted),
    clients: count(value?.clients)
  };
}

function normalizeTrusted(value: Raw<ClientStatTrusted>): ClientStatTrusted {
  return {
    name: text(value.name),
    requests: count(value.requests),
    points: normalizePoints(value.points)
  };
}

function normalizeBlock(value: Raw<ClientStatBlock> | null | undefined): ClientStatBlock | null {
  if (!value || typeof value !== "object" || !value.id) return null;
  return {
    id: String(value.id),
    ip: text(value.ip),
    reason: text(value.reason),
    source: value.source === "agent" ? "agent" : "admin",
    created_by: text(value.created_by),
    expires_at_unix: count(value.expires_at_unix)
  };
}

function normalizeItem(value: Raw<ClientStatItem>): ClientStatItem {
  return {
    ip: text(value.ip),
    trusted: text(value.trusted),
    requests: count(value.requests),
    preflights: count(value.preflights),
    blocked: count(value.blocked),
    limited: count(value.limited),
    last_rule: text(value.last_rule),
    first_seen_at_unix: count(value.first_seen_at_unix),
    last_seen_at_unix: count(value.last_seen_at_unix),
    block: normalizeBlock(value.block as Raw<ClientStatBlock> | null | undefined),
    peak_rpm: rate(value.peak_rpm),
    recent_rpm: rate(value.recent_rpm)
  };
}

function normalizeIpLimit(value: Raw<ClientStatIpLimit>): ClientStatIpLimit {
  return {
    name: text(value.name),
    requests: count(value.requests),
    period_seconds: count(value.period_seconds),
    rpm: rate(value.rpm)
  };
}

export async function getClientStatOverview(rangeSeconds: number): Promise<ClientStatOverviewRep> {
  const rep = await apiFetch<Raw<ClientStatOverviewRep>>(
    withQuery("/client_stat/overview", { range_seconds: rangeSeconds })
  );
  return {
    step_seconds: count(rep.step_seconds),
    points: normalizePoints(rep.points),
    totals: normalizeTotals(rep.totals as Raw<ClientStatTotals> | null | undefined),
    trusted: list<ClientStatTrusted>(rep.trusted)
      .map(normalizeTrusted)
      .sort((a, b) => b.requests - a.requests)
  };
}

export async function getClientStatTop(
  rangeSeconds: number,
  kind: ClientStatKind,
  limit = 50
): Promise<ClientStatTopRep> {
  const rep = await apiFetch<Raw<ClientStatTopRep>>(
    withQuery("/client_stat/top", { range_seconds: rangeSeconds, kind, limit })
  );
  return {
    results: list<ClientStatItem>(rep.results).map(normalizeItem),
    ip_limits: list<ClientStatIpLimit>(rep.ip_limits)
      .map(normalizeIpLimit)
      .filter((limit) => limit.rpm > 0)
      .sort((a, b) => a.rpm - b.rpm),
    recent_from_unix: count(rep.recent_from_unix)
  };
}

export async function getClientStatSeries(
  ip: string,
  rangeSeconds: number
): Promise<ClientStatSeriesRep> {
  const rep = await apiFetch<Raw<ClientStatSeriesRep>>(
    withQuery("/client_stat/series", { ip, range_seconds: rangeSeconds })
  );
  return { step_seconds: count(rep.step_seconds), points: normalizePoints(rep.points) };
}
