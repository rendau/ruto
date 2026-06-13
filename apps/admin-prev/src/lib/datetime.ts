function getLocale(): string {
  // if (typeof navigator !== "undefined") {
  //   if (Array.isArray(navigator.languages) && navigator.languages.length > 0) {
  //     return navigator.languages[0];
  //   }
  //   if (navigator.language) {
  //     return navigator.language;
  //   }
  // }
  return "ru-RU";
}

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

const dateTimeFormatter = new Intl.DateTimeFormat(getLocale(), {
  dateStyle: "short",
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
