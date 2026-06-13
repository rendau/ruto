import { apiFetch } from "./http";
import { withQuery } from "./query";
import { normalizeApp, serializeApp } from "./normalize";
import type {
  AppCreateRep,
  AppGetSwaggerUrlByBackendUrlRep,
  AppGrpcReflectionEndpointsRep,
  AppInheritedReq,
  AppInterpolateReq,
  AppListRep,
  AppMain,
  AppSwaggerEndpointsDiffRep
} from "./types";
import { variablesToMap } from "./normalize";

export function listApps(req: { active?: boolean } = {}): Promise<AppListRep> {
  return apiFetch<AppListRep>(withQuery("/app", { active: req.active })).then((rep) => ({
    ...rep,
    results: (rep.results || []).map(normalizeApp)
  }));
}

export function getApp(id: string): Promise<AppMain> {
  return apiFetch<AppMain>(`/app/${encodeURIComponent(id)}`).then(normalizeApp);
}

export function createApp(req: AppMain): Promise<AppCreateRep> {
  return apiFetch<AppCreateRep>("/app", { method: "POST", body: JSON.stringify(serializeApp(req)) });
}

export function updateApp(req: AppMain): Promise<void> {
  return apiFetch<void>(`/app/${encodeURIComponent(req.id)}`, {
    method: "PUT",
    body: JSON.stringify(serializeApp(req))
  });
}

export function deleteApp(id: string): Promise<void> {
  return apiFetch<void>(`/app/${encodeURIComponent(id)}`, { method: "DELETE" });
}

export function getAppSwaggerEndpointsDiff(id: string): Promise<AppSwaggerEndpointsDiffRep> {
  return apiFetch<AppSwaggerEndpointsDiffRep>(
    `/app/${encodeURIComponent(id)}/swagger/endpoints-diff`
  );
}

export function getAppGrpcReflectionEndpoints(id: string): Promise<AppGrpcReflectionEndpointsRep> {
  return apiFetch<AppGrpcReflectionEndpointsRep>(
    `/app/${encodeURIComponent(id)}/grpc/reflection/endpoints`
  );
}

export function getAppSwaggerUrlByBackendUrl(
  backendUrl: string
): Promise<AppGetSwaggerUrlByBackendUrlRep> {
  return apiFetch<AppGetSwaggerUrlByBackendUrlRep>(
    withQuery("/app/swagger/url/by-backend-url", { backend_url: backendUrl })
  );
}

export function getAppInterpolate(req: AppInterpolateReq): Promise<AppMain> {
  return apiFetch<AppMain>(`/app/${encodeURIComponent(req.id || "")}/interpolate`, {
    method: "POST",
    body: JSON.stringify({ id: req.id || "", variables: variablesToMap(req.variables) })
  }).then(normalizeApp);
}

export function getAppInherited(req: AppInheritedReq): Promise<AppMain> {
  return apiFetch<AppMain>(`/app/${encodeURIComponent(req.id || "")}/inherited`, {
    method: "POST",
    body: JSON.stringify({ id: req.id || "", variables: variablesToMap(req.variables) })
  }).then(normalizeApp);
}
