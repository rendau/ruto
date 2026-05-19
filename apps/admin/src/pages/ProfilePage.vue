<script setup lang="ts">
import { computed, ref } from "vue";
import { useAuthStore } from "../stores/auth";
import { notifyError, notifySuccess } from "../lib/notify";

const authStore = useAuthStore();
const saving = ref(false);
const errorMessage = ref("");

const name = ref(authStore.profile?.name || "");
const password = ref("");

const info = computed(() => authStore.profile);

async function submit() {
  saving.value = true;
  errorMessage.value = "";

  try {
    const payload: { name?: string; password?: string } = {
      name: name.value.trim()
    };
    if (password.value.trim()) {
      payload.password = password.value;
    }

    await authStore.updateProfile(payload);
    name.value = authStore.profile?.name || "";
    password.value = "";
    notifySuccess("Profile updated");
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : "Unable to update profile";
    notifyError(errorMessage.value);
  } finally {
    saving.value = false;
  }
}
</script>

<template>
  <section v-if="info" class="summary-grid">
    <div>
      <span class="label">Username</span>
      <strong>{{ info.username }}</strong>
    </div>
    <div>
      <span class="label">Role</span>
      <strong>{{ info.is_admin ? "admin" : "user" }}</strong>
    </div>
    <div>
      <span class="label">Status</span>
      <strong>{{ info.active ? "active" : "inactive" }}</strong>
    </div>
  </section>

  <p v-if="errorMessage" class="error">{{ errorMessage }}</p>

  <form class="stack" @submit.prevent="submit">
    <label class="field">
      <span>Name</span>
      <input v-model="name" />
    </label>
    <label class="field">
      <span>New Password</span>
      <input v-model="password" type="password" />
    </label>

    <div class="actions">
      <button class="primary-button" type="submit" :disabled="saving">
        {{ saving ? "Saving..." : "Update Profile" }}
      </button>
    </div>
  </form>
</template>
