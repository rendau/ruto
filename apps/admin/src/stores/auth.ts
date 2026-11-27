import { defineStore } from "pinia";
import { computed, ref } from "vue";
import { ApiError } from "@/api/http";
import {
  getProfile,
  login as apiLogin,
  logout as apiLogout,
  updateProfile as apiUpdateProfile
} from "@/api/usr";
import { useAppsStore } from "./apps";
import { useChangeRequestsStore } from "./changeRequests";
import { useRootStore } from "./root";
import { useSnapshotStore } from "./snapshot";
import type { UsrMain } from "@/api/types";

export const useAuthStore = defineStore("auth", () => {
  const profile = ref<UsrMain | null>(null);
  const initialized = ref(false);
  const loading = ref(false);

  const isAuthenticated = computed(() => Boolean(profile.value));
  const isAdmin = computed(() => Boolean(profile.value?.is_admin));
  // Full app access: admins and users flagged with all_apps may manage every app.
  const hasFullAppAccess = computed(() => Boolean(profile.value?.is_admin || profile.value?.all_apps));
  const ownAppIds = computed(() => profile.value?.app_ids ?? []);
  const canDeploy = computed(() => hasFullAppAccess.value || ownAppIds.value.length > 0);

  // canManageApp reports whether the current user may edit/test the given app.
  // Other apps remain visible but read-only.
  function canManageApp(appId: string): boolean {
    if (!appId) return false;
    return hasFullAppAccess.value || ownAppIds.value.includes(appId);
  }

  // initialize finds out whether the session cookie (unseen by the page) is
  // still valid.
  async function initialize(): Promise<void> {
    if (initialized.value) return;
    try {
      profile.value = await getProfile({ notifyAuth: false });
    } catch (error) {
      if (!(error instanceof ApiError && error.status === 401)) {
        throw error;
      }
      profile.value = null;
    } finally {
      initialized.value = true;
    }
  }

  async function login(username: string, password: string): Promise<void> {
    loading.value = true;
    try {
      await apiLogin(username, password);
      profile.value = await getProfile();
      initialized.value = true;
    } finally {
      loading.value = false;
    }
  }

  async function refreshProfile(): Promise<void> {
    profile.value = await getProfile();
  }

  async function updateProfile(payload: { name?: string; password?: string; current_password?: string }): Promise<void> {
    await apiUpdateProfile(payload);
    await refreshProfile();
  }

  function logout(): void {
    // the cookie is HttpOnly: only core can remove it
    void apiLogout().catch(() => undefined);
    profile.value = null;
    initialized.value = true;
    // Clear cached data so the next user starts from a clean slate.
    useAppsStore().reset();
    useRootStore().reset();
    useSnapshotStore().reset();
    useChangeRequestsStore().reset();
  }

  return {
    profile,
    initialized,
    loading,
    isAuthenticated,
    isAdmin,
    hasFullAppAccess,
    ownAppIds,
    canDeploy,
    canManageApp,
    initialize,
    login,
    refreshProfile,
    updateProfile,
    logout
  };
});
