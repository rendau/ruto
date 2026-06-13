import { defineStore } from "pinia";
import { ref } from "vue";
import { deploySnapshot, getSnapshotVersion } from "@/api/snapshot";

export const useSnapshotStore = defineStore("snapshot", () => {
  const version = ref("");
  const deploying = ref(false);

  async function loadVersion(): Promise<void> {
    try {
      const rep = await getSnapshotVersion();
      version.value = rep.version || "";
    } catch {
      version.value = "";
    }
  }

  async function deploy(): Promise<void> {
    deploying.value = true;
    try {
      await deploySnapshot();
    } finally {
      deploying.value = false;
    }
  }

  function reset(): void {
    version.value = "";
  }

  return { version, deploying, loadVersion, deploy, reset };
});
