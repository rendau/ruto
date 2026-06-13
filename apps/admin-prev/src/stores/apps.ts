import { defineStore } from "pinia";
import type { AppMain } from "../types/api";
import { listApps } from "../lib/api";

export const useAppsStore = defineStore("apps", {
  state: () => ({
    items: [] as AppMain[],
    loading: false
  }),
  actions: {
    async loadMenuApps() {
      this.loading = true;
      try {
        const rep = await listApps();
        this.items = rep.results;
      } finally {
        this.loading = false;
      }
    }
  }
});
