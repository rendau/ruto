import { ref } from "vue";
import type { AppMain } from "@/api/types";

// Module-scoped controller for the global "create / edit application" drawer,
// hosted once in DefaultLayout and opened from anywhere (sidebar, workspace).
const show = ref(false);
const app = ref<AppMain | null>(null);

export function useAppForm() {
  function open(target: AppMain | null = null): void {
    app.value = target;
    show.value = true;
  }

  function close(): void {
    show.value = false;
  }

  return { show, app, open, close };
}
