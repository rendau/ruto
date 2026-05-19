import { reactive } from "vue";

export type NoticeKind = "success" | "error";

export interface Notice {
  id: number;
  kind: NoticeKind;
  message: string;
}

const state = reactive({
  items: [] as Notice[]
});

let seq = 0;

function push(kind: NoticeKind, message: string): void {
  const id = ++seq;
  state.items.push({ id, kind, message });
  window.setTimeout(() => {
    const idx = state.items.findIndex((x) => x.id === id);
    if (idx >= 0) {
      state.items.splice(idx, 1);
    }
  }, 2600);
}

export function notifySuccess(message: string): void {
  push("success", message);
}

export function notifyError(message: string): void {
  push("error", message);
}

export function useNotices() {
  return state;
}
