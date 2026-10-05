// Mirrors the server rules for a wildcard in an HTTP endpoint path: '*' is
// accepted only as the whole last segment, and only for apps that allow it.

export function hasWildcard(path: string): boolean {
  return path.includes("*");
}

export function wildcardPathError(path: string, appAllowsWildcard: boolean): string {
  const trimmed = path.trim().replace(/^\/+|\/+$/g, "");
  if (!trimmed.includes("*")) return "";
  const wellFormed =
    trimmed === "*" || (trimmed.endsWith("/*") && trimmed.indexOf("*") === trimmed.length - 1);
  if (!wellFormed) return "wildcard '*' is allowed only as the last segment";
  if (!appAllowsWildcard) return "wildcard '*' is not allowed for this app";
  return "";
}
