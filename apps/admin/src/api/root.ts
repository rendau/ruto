import { apiFetch } from "./http";
import { withQuery } from "./query";
import { normalizeRoot, serializeRoot, variablesToMap } from "./normalize";
import type { RootInterpolateReq, RootJwtKidsRep, RootMain } from "./types";

export function getRoot(): Promise<RootMain> {
  return apiFetch<RootMain>("/root").then(normalizeRoot);
}

export function setRoot(req: RootMain): Promise<void> {
  return apiFetch<void>("/root", { method: "POST", body: JSON.stringify(serializeRoot(req)) });
}

export function getRootJwtKidsByUrls(urls: string[]): Promise<RootJwtKidsRep> {
  return apiFetch<RootJwtKidsRep>(withQuery("/root/jwt/kids/by-urls", { urls }));
}

export function getRootInterpolate(req: RootInterpolateReq): Promise<RootMain> {
  return apiFetch<RootMain>("/root/interpolate", {
    method: "POST",
    body: JSON.stringify({ variables: variablesToMap(req.variables) })
  }).then(normalizeRoot);
}
