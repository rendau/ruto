import { apiFetch } from "./http";
import { withQuery } from "./query";
import { normalizeEndpoint, serializeEndpoint, variablesToMap } from "./normalize";
import type {
  EndpointCreateRep,
  EndpointInheritedReq,
  EndpointInterpolateReq,
  EndpointListRep,
  EndpointMain,
  EndpointTestRequest,
  EndpointTestResponse
} from "./types";

export function listEndpoints(
  req: { app_id?: string; active?: boolean } = {}
): Promise<EndpointListRep> {
  return apiFetch<EndpointListRep>(
    withQuery("/endpoint", { app_id: req.app_id, active: req.active })
  ).then((rep) => ({
    ...rep,
    results: (rep.results || []).map(normalizeEndpoint)
  }));
}

export function getEndpoint(id: string): Promise<EndpointMain> {
  return apiFetch<EndpointMain>(`/endpoint/${encodeURIComponent(id)}`).then(normalizeEndpoint);
}

export function createEndpoint(req: EndpointMain): Promise<EndpointCreateRep> {
  return apiFetch<EndpointCreateRep>("/endpoint", {
    method: "POST",
    body: JSON.stringify(serializeEndpoint(req))
  });
}

export function updateEndpoint(req: EndpointMain): Promise<void> {
  return apiFetch<void>(`/endpoint/${encodeURIComponent(req.id)}`, {
    method: "PUT",
    body: JSON.stringify(serializeEndpoint(req))
  });
}

export function deleteEndpoint(id: string): Promise<void> {
  return apiFetch<void>(`/endpoint/${encodeURIComponent(id)}`, { method: "DELETE" });
}

export function getEndpointInterpolate(req: EndpointInterpolateReq): Promise<EndpointMain> {
  return apiFetch<EndpointMain>(`/endpoint/${encodeURIComponent(req.id || "")}/interpolate`, {
    method: "POST",
    body: JSON.stringify({ id: req.id || "", variables: variablesToMap(req.variables) })
  }).then(normalizeEndpoint);
}

export function getEndpointInherited(req: EndpointInheritedReq): Promise<EndpointMain> {
  return apiFetch<EndpointMain>(`/endpoint/${encodeURIComponent(req.id || "")}/inherited`, {
    method: "POST",
    body: JSON.stringify({ id: req.id || "", variables: variablesToMap(req.variables) })
  }).then(normalizeEndpoint);
}

export function testEndpointRequest(
  id: string,
  req: EndpointTestRequest
): Promise<EndpointTestResponse> {
  return apiFetch<EndpointTestResponse>(`/endpoint/${encodeURIComponent(id)}/test`, {
    method: "POST",
    body: JSON.stringify({
      id,
      path_params: (req.path_params || []).filter((item) => (item.key || "").trim() !== ""),
      query_params: (req.query_params || []).filter((item) => (item.key || "").trim() !== ""),
      body: req.body || ""
    })
  });
}
