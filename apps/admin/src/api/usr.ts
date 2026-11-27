import { apiFetch, type FetchOptions } from "./http";
import { withQuery } from "./query";
import type {
  UsrBootstrapStatusRep,
  UsrCreateRep,
  UsrCreateReq,
  UsrEditReq,
  UsrListRep,
  UsrMain
} from "./types";

// login signs in: core answers with the HttpOnly session cookie, the token
// never reaches the page.
export async function login(username: string, password: string): Promise<void> {
  await apiFetch<void>(
    "/session",
    { method: "POST", body: JSON.stringify({ username, password }) },
    { notifyAuth: false }
  );
}

export async function logout(): Promise<void> {
  await apiFetch<void>("/session", { method: "DELETE" }, { notifyAuth: false });
}

export function getProfile(options?: FetchOptions): Promise<UsrMain> {
  return apiFetch<UsrMain>("/usr/profile", undefined, options);
}

export function getBootstrapStatus(): Promise<UsrBootstrapStatusRep> {
  return apiFetch<UsrBootstrapStatusRep>("/usr/bootstrap/status");
}

export function updateProfile(req: { name?: string; password?: string; current_password?: string }): Promise<void> {
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
