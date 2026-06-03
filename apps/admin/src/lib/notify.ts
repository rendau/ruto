import type { MessageApi } from "naive-ui";

export type NoticeKind = "success" | "error";

export interface Notice {
  id: number;
  kind: NoticeKind;
  message: string;
}

let messageApi: MessageApi | null = null;

function push(kind: NoticeKind, message: string): void {
  if (messageApi) {
    messageApi[kind](message);
    return;
  }
  // Fallback for code that can run before the Naive provider is mounted.
  console[kind === "error" ? "error" : "info"](message);
}

export function notifySuccess(message: string): void {
  push("success", message);
}

export function notifyError(message: string): void {
  push("error", message);
}

export function setMessageApi(api: MessageApi): void {
  messageApi = api;
}
