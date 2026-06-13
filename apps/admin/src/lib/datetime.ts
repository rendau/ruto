const LOCALE = "en-US";

function parseUnix(value: unknown): number {
  if (typeof value === "number" && Number.isFinite(value)) {
    return value;
  }
  if (typeof value === "string" && value.trim() !== "") {
    const parsed = Number(value);
    return Number.isFinite(parsed) ? parsed : 0;
  }
  return 0;
}

const dateTimeFormatter = new Intl.DateTimeFormat(LOCALE, {
  dateStyle: "medium",
  timeStyle: "short"
});

export function formatUnixDateTime(value: unknown, fallback = "n/a"): string {
  const unix = parseUnix(value);
  if (!Number.isFinite(unix) || unix <= 0) {
    return fallback;
  }
  return dateTimeFormatter.format(new Date(unix * 1000));
}

export function formatUnixAge(value: unknown, fallback = "n/a"): string {
  const unix = parseUnix(value);
  if (!Number.isFinite(unix) || unix <= 0) {
    return fallback;
  }

  const nowUnix = Math.floor(Date.now() / 1000);
  let diff = nowUnix - Math.floor(unix);
  if (diff < 0) {
    diff = 0;
  }

  if (diff < 60) {
    return `${diff}s ago`;
  }
  if (diff < 3600) {
    return `${Math.floor(diff / 60)}m ago`;
  }
  if (diff < 86400) {
    return `${Math.floor(diff / 3600)}h ago`;
  }
  return `${Math.floor(diff / 86400)}d ago`;
}

export function formatDuration(seconds: number): string {
  const total = Math.max(0, Math.floor(seconds || 0));
  const days = Math.floor(total / 86400);
  const hours = Math.floor((total % 86400) / 3600);
  const minutes = Math.floor((total % 3600) / 60);
  const secs = total % 60;

  const parts: string[] = [];
  if (days > 0) parts.push(`${days}d`);
  if (hours > 0) parts.push(`${hours}h`);
  if (minutes > 0) parts.push(`${minutes}m`);
  if (parts.length === 0) parts.push(`${secs}s`);
  return parts.join(" ");
}
