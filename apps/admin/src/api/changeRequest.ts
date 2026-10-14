import { apiFetch } from "./http";
import { withQuery } from "./query";
import type {
  ChangeRequestChange,
  ChangeRequestDetail,
  ChangeRequestListRep,
  ChangeRequestMain,
  ChangeRequestOp,
  ChangeRequestStatus
} from "./types";

// int64 fields arrive as strings from grpc-gateway.
function normalizeMain(value: ChangeRequestMain): ChangeRequestMain {
  return {
    id: Number(value?.id || 0),
    status: value?.status || "pending",
    title: value?.title || "",
    description: value?.description || "",
    publish: Boolean(value?.publish),
    ops_count: Number(value?.ops_count || 0),
    created_at_unix: Number(value?.created_at_unix || 0),
    api_key_name: value?.api_key_name || "",
    decided_at_unix: Number(value?.decided_at_unix || 0),
    comment: value?.comment || "",
    error: value?.error || "",
    published_at_unix: Number(value?.published_at_unix || 0),
    publish_error: value?.publish_error || ""
  };
}

function normalizeChange(value: ChangeRequestChange): ChangeRequestChange {
  return {
    path: value?.path || "",
    before: value?.before || "",
    after: value?.after || "",
    secret: Boolean(value?.secret)
  };
}

function normalizeOp(value: ChangeRequestOp): ChangeRequestOp {
  return {
    op: value?.op,
    target_kind: value?.target_kind || "",
    target_id: value?.target_id || "",
    title: value?.title || "",
    app_name: value?.app_name || "",
    changes: (value?.changes || []).map(normalizeChange),
    notes: value?.notes || []
  };
}

export interface ChangeRequestListReq {
  status?: ChangeRequestStatus;
  page?: number;
  page_size?: number;
  with_total_count?: boolean;
}

export function listChangeRequests(req: ChangeRequestListReq = {}): Promise<ChangeRequestListRep> {
  return apiFetch<ChangeRequestListRep>(
    withQuery("/change_request", {
      "list_params.page": req.page,
      "list_params.page_size": req.page_size,
      "list_params.with_total_count": req.with_total_count,
      status: req.status
    })
  ).then((rep) => ({
    pagination_info: rep.pagination_info,
    results: (rep.results || []).map(normalizeMain)
  }));
}

export function countPendingChangeRequests(): Promise<number> {
  return apiFetch<ChangeRequestListRep>(
    withQuery("/change_request", {
      status: "pending",
      "list_params.only_count": true,
      "list_params.with_total_count": true
    })
  ).then((rep) => Number(rep.pagination_info?.total_count || 0));
}

export function getChangeRequest(id: number): Promise<ChangeRequestDetail> {
  return apiFetch<ChangeRequestDetail>(`/change_request/${id}`).then((rep) => ({
    main: normalizeMain(rep.main),
    author_name: rep.author_name || "",
    decided_name: rep.decided_name || "",
    outdated: Boolean(rep.outdated),
    outdated_reason: rep.outdated_reason || "",
    ops: (rep.ops || []).map(normalizeOp)
  }));
}

export function applyChangeRequest(id: number, publish: boolean): Promise<void> {
  return apiFetch<void>(`/change_request/${id}/apply`, {
    method: "POST",
    body: JSON.stringify({ publish })
  });
}

export function rejectChangeRequest(id: number, comment: string): Promise<void> {
  return apiFetch<void>(`/change_request/${id}/reject`, {
    method: "POST",
    body: JSON.stringify({ comment })
  });
}
