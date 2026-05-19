import { defineStore } from "pinia";
import type { UsrMain } from "../types/api";
import { getProfile, login as apiLogin, logout as apiLogout, updateProfile as apiUpdateProfile } from "../lib/api";
import { getToken } from "../lib/auth-session";

interface LoginPayload {
  username: string;
  password: string;
}

export const useAuthStore = defineStore("auth", {
  state: () => ({
    token: getToken(),
    profile: null as UsrMain | null,
    initialized: false,
    loading: false
  }),
  getters: {
    isAuthenticated: (state) => !!state.token && !!state.profile
  },
  actions: {
    syncToken() {
      this.token = getToken();
    },
    async initialize() {
      if (this.initialized) {
        return;
      }
      this.syncToken();
      if (!this.token) {
        this.initialized = true;
        return;
      }
      try {
        this.profile = await getProfile();
      } finally {
        this.syncToken();
        this.initialized = true;
      }
    },
    async login(payload: LoginPayload) {
      this.loading = true;
      try {
        await apiLogin(payload.username, payload.password);
        this.syncToken();
        this.profile = await getProfile();
      } finally {
        this.loading = false;
      }
    },
    async refreshProfile() {
      this.profile = await getProfile();
      this.syncToken();
    },
    async updateProfile(payload: { name?: string; password?: string }) {
      await apiUpdateProfile(payload);
      await this.refreshProfile();
    },
    logout() {
      apiLogout();
      this.token = "";
      this.profile = null;
      this.initialized = true;
    }
  }
});
