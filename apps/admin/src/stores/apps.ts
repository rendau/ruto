import { defineStore } from "pinia";
import { computed, ref } from "vue";
import { listApps } from "@/api/app";
import type { AppMain } from "@/api/types";

export const useAppsStore = defineStore("apps", () => {
  const apps = ref<AppMain[]>([]);
  const loading = ref(false);
  const loaded = ref(false);

  const count = computed(() => apps.value.length);

  // Distinct group names across all apps, for the group picker in the app form.
  const groups = computed(() =>
    Array.from(new Set(apps.value.map((app) => app.group.trim()).filter(Boolean))).sort((a, b) =>
      a.localeCompare(b)
    )
  );

  async function load(): Promise<void> {
    loading.value = true;
    try {
      const rep = await listApps();
      apps.value = rep.results ?? [];
      loaded.value = true;
    } finally {
      loading.value = false;
    }
  }

  async function ensureLoaded(): Promise<void> {
    if (!loaded.value) {
      await load();
    }
  }

  function getById(id: string): AppMain | null {
    return apps.value.find((app) => app.id === id) ?? null;
  }

  function reset(): void {
    apps.value = [];
    loaded.value = false;
    loading.value = false;
  }

  return { apps, loading, loaded, count, groups, load, refresh: load, ensureLoaded, getById, reset };
});
