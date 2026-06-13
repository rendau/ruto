import { defineStore } from "pinia";
import { computed, ref } from "vue";
import { getToken } from "@/api/auth-session";
import {
  getProfile,
  login as apiLogin,
  logout as apiLogout,
  updateProfile as apiUpdateProfile
} from "@/api/usr";
import type { UsrMain } from "@/api/types";

export const useAuthStore = defineStore("auth", () => {
  const token = ref(getToken());
  const profile = ref<UsrMain | null>(null);
  const initialized = ref(false);
  const loading = ref(false);

  const isAuthenticated = computed(() => Boolean(token.value) && Boolean(profile.value));
  const isAdmin = computed(() => Boolean(profile.value?.is_admin));

  function syncToken(): void {
    token.value = getToken();
  }

  async function initialize(): Promise<void> {
    if (initialized.value) return;
    syncToken();
    if (!token.value) {
      initialized.value = true;
      return;
    }
    try {
      profile.value = await getProfile();
    } finally {
      syncToken();
      initialized.value = true;
    }
  }

  async function login(username: string, password: string): Promise<void> {
    loading.value = true;
    try {
      await apiLogin(username, password);
      syncToken();
      profile.value = await getProfile();
      initialized.value = true;
    } finally {
      loading.value = false;
    }
  }

  async function refreshProfile(): Promise<void> {
    profile.value = await getProfile();
    syncToken();
  }

  async function updateProfile(payload: { name?: string; password?: string }): Promise<void> {
    await apiUpdateProfile(payload);
    await refreshProfile();
  }

  function logout(): void {
    apiLogout();
    token.value = "";
    profile.value = null;
    initialized.value = true;
  }

  return {
    token,
    profile,
    initialized,
    loading,
    isAuthenticated,
    isAdmin,
    initialize,
    login,
    refreshProfile,
    updateProfile,
    logout
  };
});
