import type { ChangeRequestOpKind, ChangeRequestStatus } from "@/api/types";
import type { TagType } from "@/constants/enums";

export const CHANGE_REQUEST_STATUS_LABEL: Record<ChangeRequestStatus, string> = {
  pending: "Pending",
  applied: "Applied",
  rejected: "Rejected",
  cancelled: "Cancelled",
  expired: "Expired",
  outdated: "Outdated"
};

const STATUS_TAG: Record<ChangeRequestStatus, TagType> = {
  pending: "warning",
  applied: "success",
  rejected: "error",
  outdated: "error",
  expired: "error",
  cancelled: "default"
};

export function changeRequestStatusLabel(status: string): string {
  return CHANGE_REQUEST_STATUS_LABEL[status as ChangeRequestStatus] ?? status;
}

export function changeRequestStatusTag(status: string): TagType {
  return STATUS_TAG[status as ChangeRequestStatus] ?? "default";
}

const OP_LABEL: Record<ChangeRequestOpKind, string> = {
  create_app: "New app",
  update_app: "Update app",
  delete_app: "Delete app",
  create_endpoint: "New endpoint",
  update_endpoint: "Update endpoint",
  delete_endpoint: "Delete endpoint",
  update_root: "Update root",
  delete_seen_paths: "Delete seen paths"
};

export function changeRequestOpLabel(op: string): string {
  return OP_LABEL[op as ChangeRequestOpKind] ?? op;
}

export type ChangeRequestOpTone = "create" | "update" | "delete";

export function changeRequestOpTone(op: string): ChangeRequestOpTone {
  if (op.startsWith("create_")) return "create";
  if (op.startsWith("delete_")) return "delete";
  return "update";
}
