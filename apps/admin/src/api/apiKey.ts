import { apiFetch } from "./http";
import type { ApiKeyCreateRep, ApiKeyCreateReq, ApiKeyListRep, ApiKeyMain } from "./types";

// int64 fields arrive as strings from grpc-gateway.
function normalizeApiKey(value: ApiKeyMain): ApiKeyMain {
  return {
    id: value?.id || "",
    name: value?.name || "",
    key_prefix: value?.key_prefix || "",
    created_at_unix: Number(value?.created_at_unix || 0),
    last_used_at_unix: Number(value?.last_used_at_unix || 0),
    // keys made before scopes existed have none and are full
    scope: value?.scope === "blocklist" ? "blocklist" : "full"
  };
}

export function listApiKeys(): Promise<ApiKeyListRep> {
  return apiFetch<ApiKeyListRep>("/api_key").then((rep) => ({
    results: (rep.results || []).map(normalizeApiKey)
  }));
}

export function createApiKey(req: ApiKeyCreateReq): Promise<ApiKeyCreateRep> {
  return apiFetch<ApiKeyCreateRep>("/api_key", {
    method: "POST",
    body: JSON.stringify(req)
  });
}

export function deleteApiKey(id: string): Promise<void> {
  return apiFetch<void>(`/api_key/${encodeURIComponent(id)}`, { method: "DELETE" });
}
