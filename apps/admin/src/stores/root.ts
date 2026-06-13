import { defineStore } from "pinia";
import { computed, ref } from "vue";
import { getRoot, getRootJwtKidsByUrls } from "@/api/root";
import type { RootMain } from "@/api/types";

export const useRootStore = defineStore("root", () => {
  const root = ref<RootMain | null>(null);
  const loading = ref(false);
  const loaded = ref(false);
  const jwtKids = ref<string[]>([]);

  const baseUrl = computed(() => root.value?.base_url || "");

  async function loadJwtKids(urls?: string[]): Promise<void> {
    const list = (urls ?? (root.value?.jwt || []).map((item) => item.jwk_url))
      .map((url) => url.trim())
      .filter(Boolean);
    if (list.length === 0) {
      jwtKids.value = [];
      return;
    }
    try {
      const rep = await getRootJwtKidsByUrls(list);
      jwtKids.value = rep.kids || [];
    } catch {
      jwtKids.value = [];
    }
  }

  async function load(): Promise<void> {
    loading.value = true;
    try {
      root.value = await getRoot();
      loaded.value = true;
      void loadJwtKids();
    } finally {
      loading.value = false;
    }
  }

  async function ensureLoaded(): Promise<void> {
    if (!loaded.value) {
      await load();
    }
  }

  function setRoot(value: RootMain): void {
    root.value = value;
  }

  function reset(): void {
    root.value = null;
    loaded.value = false;
    loading.value = false;
    jwtKids.value = [];
  }

  return {
    root,
    loading,
    loaded,
    jwtKids,
    baseUrl,
    load,
    refresh: load,
    ensureLoaded,
    loadJwtKids,
    setRoot,
    reset
  };
});
