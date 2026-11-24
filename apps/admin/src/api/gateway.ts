import { apiFetch } from "./http";
import type { GatewayStateListRep } from "./types";

// blocklist_version is omitted while a gateway hasn't reported one.
export function listGateways(): Promise<GatewayStateListRep> {
  return apiFetch<GatewayStateListRep>("/gateway").then((rep) => ({
    results: (rep.results || []).map((item) => ({
      ...item,
      blocklist_version: item.blocklist_version || ""
    }))
  }));
}
