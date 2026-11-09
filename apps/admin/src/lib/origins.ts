const ANY_ORIGIN = "*";
const WILDCARD = "*.";

// originAllowed mirrors the gateway's OriginMatcher: an origin is admitted by
// allow_origins when the list has "*", names it exactly, or has a subdomain
// pattern "scheme://*.domain[:port]" the origin falls under (with at least one
// more label in front).
export function originAllowed(origin: string, allowOrigins: string[]): boolean {
  const needle = origin.trim().toLowerCase();
  for (const raw of allowOrigins) {
    const entry = raw.trim().toLowerCase();
    if (!entry) continue;
    if (entry === ANY_ORIGIN || entry === needle) return true;

    const sep = entry.indexOf("://");
    if (sep < 0) continue;
    const scheme = entry.slice(0, sep + 3);
    const host = entry.slice(sep + 3);
    if (!host.startsWith(WILDCARD)) continue;

    const suffix = host.slice(WILDCARD.length - 1);
    if (!needle.startsWith(scheme)) continue;
    const originHost = needle.slice(scheme.length);
    if (originHost.length > suffix.length && originHost.endsWith(suffix)) return true;
  }
  return false;
}
