import { apiFetch } from "./http";
import type { SeenOriginListRep } from "./types";

export function listSeenOrigins(): Promise<SeenOriginListRep> {
  return apiFetch<SeenOriginListRep>("/seen_origin").then((rep) => ({
    results: (rep.results || []).map((item) => ({ ...item, hits: Number(item.hits || 0) }))
  }));
}

export function deleteSeenOrigins(origins: string[]): Promise<void> {
  return apiFetch<void>("/seen_origin/delete", {
    method: "POST",
    body: JSON.stringify({ origins })
  });
}

export function clearSeenOrigins(): Promise<void> {
  return apiFetch<void>("/seen_origin", { method: "DELETE" });
}
