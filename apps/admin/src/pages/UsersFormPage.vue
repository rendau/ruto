<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { createUser, getUser, listApps, updateUser } from "../lib/api";
import { notifyError, notifySuccess } from "../lib/notify";
import { useAuthStore } from "../stores/auth";
import type { AppMain, UsrCreateReq, UsrEditReq } from "../types/api";

interface UserForm {
  id: number;
  active: boolean;
  is_admin: boolean;
  all_apps: boolean;
  app_ids: string[];
  name: string;
  username: string;
  password: string;
}

const route = useRoute();
const router = useRouter();
const authStore = useAuthStore();

const loading = ref(false);
const saving = ref(false);
const errorMessage = ref("");
const allApps = ref<AppMain[]>([]);

const form = ref<UserForm>({
  id: 0,
  active: true,
  is_admin: false,
  all_apps: false,
  app_ids: [],
  name: "",
  username: "",
  password: ""
});

const isEdit = computed(() => typeof route.params.id === "string" && route.params.id.length > 0);
const canManage = computed(() => Boolean(authStore.profile?.is_admin));
const entityId = computed(() => {
  const raw = typeof route.params.id === "string" ? route.params.id : "";
  const parsed = Number(raw);
  return Number.isFinite(parsed) ? parsed : 0;
});

const appOptions = computed(() =>
  allApps.value.map((app) => ({
    label: app.name || app.id,
    value: app.id
  }))
);

const showAppAccess = computed(() => !form.value.is_admin);

async function loadUser() {
  if (!isEdit.value || !entityId.value) {
    return;
  }
  loading.value = true;
  errorMessage.value = "";
  try {
    const user = await getUser(entityId.value);
    form.value = {
      id: entityId.value,
      active: Boolean(user.active),
      is_admin: Boolean(user.is_admin),
      all_apps: Boolean(user.all_apps),
      app_ids: Array.isArray(user.app_ids) ? user.app_ids : [],
      name: user.name || "",
      username: user.username || "",
      password: ""
    };
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : "Unable to load user";
    notifyError(errorMessage.value);
  } finally {
    loading.value = false;
  }
}

async function loadAppsList() {
  try {
    const rep = await listApps();
    allApps.value = rep.results || [];
  } catch {
    // non-critical
  }
}

async function submitForm() {
  if (!canManage.value) {
    notifyError("Admin permissions are required");
    return;
  }
  if (!isEdit.value && !form.value.password.trim()) {
    notifyError("Password is required");
    return;
  }

  saving.value = true;
  errorMessage.value = "";
  try {
    if (isEdit.value) {
      const payload: UsrEditReq = {
        id: entityId.value,
        active: form.value.active,
        is_admin: form.value.is_admin,
        all_apps: form.value.is_admin ? undefined : form.value.all_apps,
        update_app_ids: !form.value.is_admin,
        app_ids: form.value.is_admin ? undefined : form.value.app_ids,
        name: form.value.name.trim(),
        username: form.value.username.trim()
      };
      await updateUser(payload);
      notifySuccess("User updated");
    } else {
      const payload: UsrCreateReq = {
        active: form.value.active,
        is_admin: form.value.is_admin,
        all_apps: form.value.is_admin ? undefined : form.value.all_apps,
        app_ids: form.value.is_admin ? undefined : form.value.app_ids,
        name: form.value.name.trim(),
        username: form.value.username.trim(),
        password: form.value.password
      };
      await createUser(payload);
      notifySuccess("User created");
    }

    await router.push({ name: "users" });
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : "Unable to save user";
    notifyError(errorMessage.value);
  } finally {
    saving.value = false;
  }
}

async function changePassword() {
  if (!canManage.value) {
    notifyError("Admin permissions are required");
    return;
  }
  if (!isEdit.value || !entityId.value) {
    notifyError("Select user first");
    return;
  }

  const nextPassword = window.prompt(`New password for "${form.value.username}":`);
  if (nextPassword === null) {
    return;
  }
  if (!nextPassword.trim()) {
    notifyError("Password is required");
    return;
  }

  saving.value = true;
  errorMessage.value = "";
  try {
    await updateUser({
      id: entityId.value,
      password: nextPassword
    });
    notifySuccess("Password updated");
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : "Unable to update password";
    notifyError(errorMessage.value);
  } finally {
    saving.value = false;
  }
}

onMounted(() => {
  void loadUser();
  void loadAppsList();
});
</script>

<template>
  <n-alert v-if="errorMessage" class="form-alert" type="error" :show-icon="false">{{ errorMessage }}</n-alert>
  <p v-if="loading" class="muted">Loading...</p>

  <form v-else class="stack" @submit.prevent="submitForm">
    <n-space>
      <n-switch v-model:value="form.active">
        <template #checked>Active</template>
        <template #unchecked>Inactive</template>
      </n-switch>
      <n-checkbox v-model:checked="form.is_admin">Admin</n-checkbox>
    </n-space>
    <label class="field">
      <span>Name</span>
      <n-input v-model:value="form.name" required />
    </label>
    <label class="field">
      <span>Username</span>
      <n-input v-model:value="form.username" required />
    </label>
    <label v-if="!isEdit" class="field">
      <span>Password</span>
      <n-input v-model:value="form.password" type="password" show-password-on="click" required />
    </label>

    <template v-if="showAppAccess">
      <n-divider />
      <div class="field">
        <span class="field-label">App Access</span>
        <n-checkbox v-model:checked="form.all_apps">All apps</n-checkbox>
      </div>
      <div v-if="!form.all_apps" class="field">
        <span>Apps</span>
        <n-select
          v-model:value="form.app_ids"
          multiple
          filterable
          clearable
          :options="appOptions"
          placeholder="Select apps..."
        />
      </div>
    </template>

    <div class="actions" style="margin-top: 0.6rem;">
      <n-button type="primary" attr-type="submit" :loading="saving">{{ saving ? "Saving..." : "Save" }}</n-button>
      <n-button v-if="isEdit" secondary :disabled="saving" @click="changePassword">Change Password</n-button>
      <n-button secondary :disabled="saving" @click="router.push({ name: 'users' })">Cancel</n-button>
    </div>
  </form>
</template>
