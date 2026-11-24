import { apiFetch } from "./http";
import { withQuery } from "./query";
import { normalizeBlockedClient } from "./normalize";
import type {
  BlockedClientCreateReq,
  BlockedClientListRep,
  BlockedClientMain,
  BlockedClientRemoveAgentBlocksRep,
  BlockedClientRemoveAgentBlocksReq
} from "./types";

export function listBlockedClients(withHistory: boolean): Promise<BlockedClientListRep> {
  return apiFetch<BlockedClientListRep>(
    withQuery("/blocked_client", { with_history: withHistory })
  ).then((rep) => ({ results: (rep.results || []).map(normalizeBlockedClient) }));
}

export function createBlockedClient(req: BlockedClientCreateReq): Promise<BlockedClientMain> {
  return apiFetch<BlockedClientMain>("/blocked_client", {
    method: "POST",
    body: JSON.stringify(req)
  }).then(normalizeBlockedClient);
}

export function removeBlockedClient(id: string, reason: string): Promise<void> {
  return apiFetch<void>(`/blocked_client/${encodeURIComponent(id)}/remove`, {
    method: "POST",
    body: JSON.stringify({ reason })
  });
}

// The kill switch: lifts every active agent block, or only those of one key.
export function removeAgentBlocks(
  req: BlockedClientRemoveAgentBlocksReq
): Promise<BlockedClientRemoveAgentBlocksRep> {
  return apiFetch<{ removed?: string | number }>("/blocked_client/remove_agent_blocks", {
    method: "POST",
    body: JSON.stringify(req)
  }).then((rep) => ({ removed: Math.max(0, Math.trunc(Number(rep.removed) || 0)) }));
}
