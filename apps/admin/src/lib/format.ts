export function formatBytes(bytes: number): string {
  const value = Number(bytes) || 0;
  if (value <= 0) {
    return "0 B";
  }
  const units = ["B", "KiB", "MiB", "GiB", "TiB"];
  const exponent = Math.min(units.length - 1, Math.floor(Math.log(value) / Math.log(1024)));
  const size = value / 1024 ** exponent;
  const rounded = size >= 100 || exponent === 0 ? Math.round(size) : Math.round(size * 10) / 10;
  return `${rounded} ${units[exponent]}`;
}

export function stripScheme(url?: string): string {
  if (!url) {
    return "";
  }
  return url.replace(/^https?:\/\//i, "");
}

export function joinUrl(baseUrl: string, path: string): string {
  const base = (baseUrl || "").trim().replace(/\/+$/g, "");
  const tail = (path || "").trim();
  if (!base) {
    return tail;
  }
  if (!tail) {
    return base;
  }
  return `${base}${tail.startsWith("/") ? "" : "/"}${tail}`;
}

export function joinPath(...segments: Array<string | undefined>): string {
  const cleaned = segments
    .map((segment) => (segment || "").trim())
    .filter(Boolean)
    .map((segment) => segment.replace(/^\/+|\/+$/g, ""))
    .filter(Boolean);
  if (cleaned.length === 0) {
    return "/";
  }
  return `/${cleaned.join("/")}`;
}

export function prettyJson(value: unknown): string {
  try {
    return JSON.stringify(value, null, 2);
  } catch {
    return String(value ?? "");
  }
}

export function maybePrettyJson(raw: string): string {
  if (!raw) {
    return "";
  }
  try {
    return JSON.stringify(JSON.parse(raw), null, 2);
  } catch {
    return raw;
  }
}

export function generateSecret(length = 32): string {
  const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789";
  const bytes = new Uint8Array(length);
  crypto.getRandomValues(bytes);
  let result = "";
  for (let i = 0; i < length; i += 1) {
    result += charset.charAt((bytes[i] ?? 0) % charset.length);
  }
  return result;
}
