import { API_BASE_URL } from "./config";
import { apiFetch, ApiError } from "./http";
import { withQuery } from "./query";
import { clearSession, setCredentials, setToken } from "./auth-session";
import type {
  ErrorRep,
  UsrBootstrapStatusRep,
  UsrCreateRep,
  UsrCreateReq,
  UsrEditReq,
  UsrListRep,
  UsrLoginRep,
  UsrMain
} from "./types";

export async function login(username: string, password: string): Promise<void> {
  const response = await fetch(`${API_BASE_URL}/usr/login`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ username, password })
  });
  const payload = (await response.json().catch(() => ({}))) as Partial<UsrLoginRep & ErrorRep>;
  if (!response.ok || !payload.jwt) {
    throw new ApiError(
      payload.message || "Login failed",
      payload.code || "not_authorized",
      response.status
    );
  }
  setToken(payload.jwt);
  setCredentials(username, password);
}

export function logout(): void {
  clearSession();
}

export function getProfile(): Promise<UsrMain> {
  return apiFetch<UsrMain>("/usr/profile");
}

export function getBootstrapStatus(): Promise<UsrBootstrapStatusRep> {
  return apiFetch<UsrBootstrapStatusRep>("/usr/bootstrap/status");
}

export function updateProfile(req: { name?: string; password?: string }): Promise<void> {
  return apiFetch<void>("/usr/profile", { method: "PUT", body: JSON.stringify(req) });
}

export interface UsrListReq {
  search?: string;
  page?: number;
  page_size?: number;
  with_total_count?: boolean;
}

export function listUsers(req: UsrListReq = {}): Promise<UsrListRep> {
  return apiFetch<UsrListRep>(
    withQuery("/usr", {
      "list_params.page": req.page,
      "list_params.page_size": req.page_size,
      "list_params.with_total_count": req.with_total_count,
      search: req.search
    })
  );
}

export function getUser(id: number): Promise<UsrMain> {
  return apiFetch<UsrMain>(`/usr/${id}`);
}

export function createUser(req: UsrCreateReq): Promise<UsrCreateRep> {
  return apiFetch<UsrCreateRep>("/usr", { method: "POST", body: JSON.stringify(req) });
}

export function updateUser(req: UsrEditReq): Promise<void> {
  return apiFetch<void>(`/usr/${req.id}`, { method: "PUT", body: JSON.stringify(req) });
}

export function deleteUser(id: number): Promise<void> {
  return apiFetch<void>(`/usr/${id}`, { method: "DELETE" });
}
