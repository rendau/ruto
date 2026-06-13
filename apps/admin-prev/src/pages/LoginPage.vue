<script setup lang="ts">
import { onMounted, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { ApiError, createUser, getBootstrapStatus } from "../lib/api";
import { useAuthStore } from "../stores/auth";

const authStore = useAuthStore();
const route = useRoute();
const router = useRouter();

const username = ref("");
const password = ref("");
const name = ref("");
const errorMessage = ref("");
const successMessage = ref("");
const bootstrapLoading = ref(false);
const bootstrapStatusLoading = ref(true);
const bootstrapAvailable = ref(false);

async function submit() {
  errorMessage.value = "";
  successMessage.value = "";
  try {
    await authStore.login({
      username: username.value,
      password: password.value
    });
    const redirect = typeof route.query.redirect === "string" ? route.query.redirect : "/";
    await router.push(redirect);
  } catch (error) {
    if (error instanceof ApiError) {
      errorMessage.value = error.message;
      return;
    }
    errorMessage.value = "Unable to login";
  }
}

async function submitBootstrap() {
  errorMessage.value = "";
  successMessage.value = "";
  bootstrapLoading.value = true;
  try {
    await createUser({
      active: true,
      is_admin: true,
      name: name.value,
      username: username.value,
      password: password.value
    });
    successMessage.value = "Первый администратор создан. Теперь выполните вход.";
    bootstrapAvailable.value = false;
    password.value = "";
  } catch (error) {
    if (error instanceof ApiError) {
      errorMessage.value = error.message;
      return;
    }
    errorMessage.value = "Unable to create first admin";
  } finally {
    bootstrapLoading.value = false;
  }
}

onMounted(async () => {
  try {
    const status = await getBootstrapStatus();
    bootstrapAvailable.value = status.can_create_first_admin;
  } catch {
    bootstrapAvailable.value = false;
  } finally {
    bootstrapStatusLoading.value = false;
  }
});
</script>

<template>
  <main class="login-page">
    <form v-if="bootstrapStatusLoading" class="login-form">
      <div class="login-brand">
        <img class="login-brand-logo" src="/ruto_logo.svg" alt="Ruto logo" />
        <h1>Ruto Admin</h1>
      </div>
      <p class="hint">Loading...</p>
    </form>

    <form v-else-if="!bootstrapAvailable" class="login-form" @submit.prevent="submit">
      <div class="login-brand">
        <img class="login-brand-logo" src="/ruto_logo.svg" alt="Ruto logo" />
        <h1>Ruto Admin</h1>
      </div>
      <label class="field">
        <span>Username</span>
        <n-input v-model:value="username" autocomplete="username" required />
      </label>
      <label class="field">
        <span>Password</span>
        <n-input v-model:value="password" type="password" show-password-on="click" autocomplete="current-password" required />
      </label>
      <n-button :loading="authStore.loading" type="primary" attr-type="submit" block>
        {{ authStore.loading ? "Signing in..." : "Sign in" }}
      </n-button>
      <n-alert v-if="errorMessage" class="form-alert" type="error" :show-icon="false">{{ errorMessage }}</n-alert>
      <n-alert v-if="successMessage" class="form-alert" type="success" :show-icon="false">{{ successMessage }}</n-alert>
      <p class="hint">Token renewal on expiry uses saved credentials (one attempt).</p>
    </form>

    <form v-else class="login-form" @submit.prevent="submitBootstrap">
      <div class="login-brand">
        <img class="login-brand-logo" src="/ruto_logo.svg" alt="Ruto logo" />
        <h1>First Admin Setup</h1>
      </div>
      <label class="field">
        <span>Name</span>
        <n-input v-model:value="name" autocomplete="name" required />
      </label>
      <label class="field">
        <span>Username</span>
        <n-input v-model:value="username" autocomplete="username" required />
      </label>
      <label class="field">
        <span>Password</span>
        <n-input v-model:value="password" type="password" show-password-on="click" autocomplete="new-password" required />
      </label>
      <n-button :loading="bootstrapLoading" type="primary" attr-type="submit" block>
        {{ bootstrapLoading ? "Creating..." : "Create admin" }}
      </n-button>
      <n-alert v-if="errorMessage" class="form-alert" type="error" :show-icon="false">{{ errorMessage }}</n-alert>
      <n-alert v-if="successMessage" class="form-alert" type="success" :show-icon="false">{{ successMessage }}</n-alert>
      <p class="hint">Доступно только при первой установке, пока в системе нет пользователей.</p>
    </form>
  </main>
</template>
