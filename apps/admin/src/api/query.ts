type QueryValue = string | number | boolean | undefined | null | string[];

export function withQuery(path: string, query: Record<string, QueryValue>): string {
  const params = new URLSearchParams();

  for (const [key, value] of Object.entries(query)) {
    if (value === undefined || value === null) {
      continue;
    }
    if (Array.isArray(value)) {
      for (const item of value) {
        params.append(key, item);
      }
      continue;
    }
    params.set(key, String(value));
  }

  const queryStr = params.toString();
  return queryStr ? `${path}?${queryStr}` : path;
}
