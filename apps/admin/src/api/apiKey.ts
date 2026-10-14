import { apiFetch } from "./http";
import type { ApiKeyCreateRep, ApiKeyListRep, ApiKeyMain } from "./types";

// int64 fields arrive as strings from grpc-gateway.
function normalizeApiKey(value: ApiKeyMain): ApiKeyMain {
  return {
    id: value?.id || "",
    name: value?.name || "",
    key_prefix: value?.key_prefix || "",
    created_at_unix: Number(value?.created_at_unix || 0),
    last_used_at_unix: Number(value?.last_used_at_unix || 0)
  };
}

export function listApiKeys(): Promise<ApiKeyListRep> {
  return apiFetch<ApiKeyListRep>("/api_key").then((rep) => ({
    results: (rep.results || []).map(normalizeApiKey)
  }));
}

export function createApiKey(name: string): Promise<ApiKeyCreateRep> {
  return apiFetch<ApiKeyCreateRep>("/api_key", {
    method: "POST",
    body: JSON.stringify({ name })
  });
}

export function deleteApiKey(id: string): Promise<void> {
  return apiFetch<void>(`/api_key/${encodeURIComponent(id)}`, { method: "DELETE" });
}
