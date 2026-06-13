import { apiFetch } from "./http";
import type { SnapshotVersionRep } from "./types";

export function getSnapshotVersion(): Promise<SnapshotVersionRep> {
  return apiFetch<SnapshotVersionRep>("/snapshot/version");
}

export function deploySnapshot(): Promise<void> {
  return apiFetch<void>("/snapshot/deploy", { method: "POST" });
}
