import { apiFetch } from "./http";
import { withQuery } from "./query";
import type { SeenPathKey, SeenPathListRep } from "./types";

export function listSeenPaths(appId: string): Promise<SeenPathListRep> {
  return apiFetch<SeenPathListRep>(withQuery("/seen_path", { app_id: appId })).then((rep) => ({
    results: (rep.results || []).map((item) => ({
      ...item,
      hits: Number(item.hits || 0),
      hits_not_found: Number(item.hits_not_found || 0)
    }))
  }));
}

export function deleteSeenPaths(appId: string, items: SeenPathKey[]): Promise<void> {
  return apiFetch<void>("/seen_path/delete", {
    method: "POST",
    body: JSON.stringify({ app_id: appId, items })
  });
}

export function clearSeenPaths(appId: string): Promise<void> {
  return apiFetch<void>(withQuery("/seen_path", { app_id: appId }), { method: "DELETE" });
}
