import type { ClientStatPoint } from "@/api/types";

export const CLIENT_STAT_RANGE_OPTIONS: { label: string; value: number }[] = [
  { label: "1h", value: 3600 },
  { label: "6h", value: 21600 },
  { label: "24h", value: 86400 },
  { label: "7d", value: 604800 }
];

export const CLIENT_STAT_DEFAULT_RANGE = 86400;

// Buckets without traffic may be left out of a reply: put zeros in their place
// so a client that called twice in a day draws as two spikes, not as a line
// between them. Points that don't sit on the step grid are kept as they are.
export function fillClientStatPoints(
  points: ClientStatPoint[],
  stepSeconds: number,
  rangeSeconds: number,
  nowUnix = Math.floor(Date.now() / 1000)
): ClientStatPoint[] {
  if (stepSeconds <= 0 || rangeSeconds <= 0) return points;
  if (points.some((p) => p.ts % stepSeconds !== 0)) return points;
  const last = Math.floor(nowUnix / stepSeconds) * stepSeconds;
  const buckets = Math.max(1, Math.ceil(rangeSeconds / stepSeconds));
  const byTs = new Map<number, ClientStatPoint>();
  for (let i = buckets - 1; i >= 0; i -= 1) {
    const ts = last - i * stepSeconds;
    byTs.set(ts, { ts, requests: 0, blocked: 0, limited: 0, trusted: 0 });
  }
  for (const point of points) {
    byTs.set(point.ts, point);
  }
  return [...byTs.values()].sort((a, b) => a.ts - b.ts);
}

export function chartPoints(
  points: ClientStatPoint[],
  field: keyof Omit<ClientStatPoint, "ts">
): Array<{ ts: number; value: number }> {
  return points.map((p) => ({ ts: p.ts, value: p[field] }));
}

// Chart tooltips show the date only for ranges longer than 6 hours.
export function chartTsFormatter(rangeSeconds: number): (ts: number) => string {
  const withDate = rangeSeconds > 21600;
  return (ts: number) =>
    new Date(ts * 1000).toLocaleString(undefined, {
      ...(withDate ? { day: "2-digit", month: "2-digit" } : {}),
      hour: "2-digit",
      minute: "2-digit"
    });
}

export function rangeLabel(rangeSeconds: number): string {
  const option = CLIENT_STAT_RANGE_OPTIONS.find((o) => o.value === rangeSeconds);
  return option ? `last ${option.label}` : "the selected period";
}
