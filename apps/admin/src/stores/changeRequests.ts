import { defineStore } from "pinia";
import { ref } from "vue";
import { countPendingChangeRequests } from "@/api/changeRequest";

const POLL_INTERVAL_MS = 30_000;

// Number of change requests awaiting an admin decision, shown as a badge in the
// main navigation. Polled only while an admin is logged in.
export const useChangeRequestsStore = defineStore("changeRequests", () => {
  const pendingCount = ref(0);
  let timer: ReturnType<typeof setInterval> | undefined;

  async function loadPendingCount(): Promise<void> {
    try {
      pendingCount.value = await countPendingChangeRequests();
    } catch {
      // keep the last known value: a missed poll should not blink the badge
    }
  }

  function startPolling(): void {
    if (timer) return;
    void loadPendingCount();
    timer = setInterval(() => {
      if (document.hidden) return;
      void loadPendingCount();
    }, POLL_INTERVAL_MS);
  }

  function stopPolling(): void {
    clearInterval(timer);
    timer = undefined;
  }

  function reset(): void {
    stopPolling();
    pendingCount.value = 0;
  }

  return { pendingCount, loadPendingCount, startPolling, stopPolling, reset };
});
