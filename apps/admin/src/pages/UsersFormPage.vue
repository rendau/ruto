<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { createUser, getUser, updateUser } from "../lib/api";
import { notifyError, notifySuccess } from "../lib/notify";
import { useAuthStore } from "../stores/auth";
import type { UsrCreateReq, UsrEditReq } from "../types/api";

interface UserForm {
  id: number;
  active: boolean;
  is_admin: boolean;
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

const form = ref<UserForm>({
  id: 0,
  active: true,
  is_admin: false,
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
        name: form.value.name.trim(),
        username: form.value.username.trim()
      };
      await updateUser(payload);
      notifySuccess("User updated");
    } else {
      const payload: UsrCreateReq = {
        active: form.value.active,
        is_admin: form.value.is_admin,
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
});
</script>

<template>
  <p v-if="errorMessage" class="error">{{ errorMessage }}</p>
  <p v-if="loading" class="muted">Loading...</p>

  <form v-else class="stack" @submit.prevent="submitForm">
    <label class="check">
      <input v-model="form.active" type="checkbox" />
      <span>Active</span>
    </label>
    <label class="check">
      <input v-model="form.is_admin" type="checkbox" />
      <span>Admin</span>
    </label>
    <label class="field">
      <span>Name</span>
      <input v-model="form.name" required />
    </label>
    <label class="field">
      <span>Username</span>
      <input v-model="form.username" required />
    </label>
    <label v-if="!isEdit" class="field">
      <span>Password</span>
      <input v-model="form.password" type="password" required />
    </label>

    <div class="actions">
      <button class="primary-button" type="submit" :disabled="saving">{{ saving ? "Saving..." : "Save" }}</button>
      <button v-if="isEdit" class="secondary-button" type="button" :disabled="saving" @click="changePassword">Change Password</button>
      <button class="secondary-button" type="button" :disabled="saving" @click="router.push({ name: 'users' })">Cancel</button>
    </div>
  </form>
</template>
