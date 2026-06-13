import { apiFetch } from "./http";
import type { GatewayStateListRep } from "./types";

export function listGateways(): Promise<GatewayStateListRep> {
  return apiFetch<GatewayStateListRep>("/gateway");
}
