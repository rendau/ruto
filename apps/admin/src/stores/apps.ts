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
        const rep = await listApps({
          list_params: {
            page: 1,
            page_size: 100,
            sort: ["name"]
          }
        });
        this.items = rep.results;
      } finally {
        this.loading = false;
      }
    }
  }
});
