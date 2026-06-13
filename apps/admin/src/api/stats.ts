import { apiFetch } from "./http";
import type { StatsResponse } from "./types";

export function getStats(): Promise<StatsResponse> {
  return apiFetch<StatsResponse>("/stats");
}
