// Mirrors the server rules for a rate limit rule (internal/domain/ratelimit):
// requests from 1 to 1e9, period a Go duration from 1s to 24h.
import type { RateLimit, RateLimitRule } from "@/api/types";

export const RATE_LIMIT_MAX_REQUESTS = 1_000_000_000;

const NS_PER_SECOND = 1e9;
const MIN_PERIOD_NS = NS_PER_SECOND;
const MAX_PERIOD_NS = 24 * 3600 * NS_PER_SECOND;

const UNIT_NS: Record<string, number> = {
  ns: 1,
  us: 1e3,
  "µs": 1e3,
  "μs": 1e3,
  ms: 1e6,
  s: NS_PER_SECOND,
  m: 60 * NS_PER_SECOND,
  h: 3600 * NS_PER_SECOND
};

// "ms" goes before "m" so "1ms" isn't read as minutes.
const DURATION_PART = /^(\d+\.?\d*|\.\d+)(ns|us|µs|μs|ms|s|m|h)/;

// Parses a Go duration ("10s", "1m30s", "1.5h") into nanoseconds; null if malformed.
export function parseGoDuration(value: string): number | null {
  let rest = value.trim();
  let sign = 1;
  if (rest.startsWith("-") || rest.startsWith("+")) {
    sign = rest.startsWith("-") ? -1 : 1;
    rest = rest.slice(1);
  }
  if (rest === "0") return 0;
  if (!rest) return null;
  let total = 0;
  while (rest) {
    const match = DURATION_PART.exec(rest);
    const [part, amount, unit] = match ?? [];
    if (!part || !amount || !unit) return null;
    total += parseFloat(amount) * (UNIT_NS[unit] ?? 0);
    rest = rest.slice(part.length);
  }
  return sign * total;
}

export interface RateLimitRuleErrors {
  requests: string;
  period: string;
}

export function rateLimitRuleErrors(rule: RateLimitRule): RateLimitRuleErrors {
  const errors: RateLimitRuleErrors = { requests: "", period: "" };
  if (!Number.isInteger(rule.requests) || rule.requests < 1 || rule.requests > RATE_LIMIT_MAX_REQUESTS) {
    errors.requests = "requests must be from 1 to 1000000000";
  }
  const period = rule.period.trim();
  if (!period) {
    errors.period = "period is required";
  } else {
    const ns = parseGoDuration(period);
    if (ns === null) {
      errors.period = "period must be a duration like 10s, 1m or 1h";
    } else if (ns < MIN_PERIOD_NS || ns > MAX_PERIOD_NS) {
      errors.period = "period must be from 1s to 24h";
    } else if (ns % MIN_PERIOD_NS !== 0) {
      errors.period = "period must be whole seconds";
    }
  }
  return errors;
}

// The first problem of the section as one line for a toast; "" when it's valid.
export function rateLimitError(value: RateLimit): string {
  for (const [index, rule] of value.rules.entries()) {
    const errors = rateLimitRuleErrors(rule);
    const problem = errors.requests || errors.period;
    if (problem) {
      const name = rule.name.trim();
      return `Rate limit, rule ${index + 1}${name ? ` (${name})` : ""}: ${problem}`;
    }
  }
  return "";
}
