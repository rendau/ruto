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
        <img class="login-brand-logo" src="/logo-ruto.svg" alt="Ruto logo" />
        <h1>Ruto Admin</h1>
      </div>
      <p class="hint">Loading...</p>
    </form>

    <form v-else-if="!bootstrapAvailable" class="login-form" @submit.prevent="submit">
      <div class="login-brand">
        <img class="login-brand-logo" src="/logo-ruto.svg" alt="Ruto logo" />
        <h1>Ruto Admin</h1>
      </div>
      <label class="field">
        <span>Username</span>
        <input v-model="username" autocomplete="username" required />
      </label>
      <label class="field">
        <span>Password</span>
        <input v-model="password" type="password" autocomplete="current-password" required />
      </label>
      <button :disabled="authStore.loading" class="primary-button" type="submit">
        {{ authStore.loading ? "Signing in..." : "Sign in" }}
      </button>
      <p v-if="errorMessage" class="error">{{ errorMessage }}</p>
      <p v-if="successMessage" class="success">{{ successMessage }}</p>
      <p class="hint">Token renewal on expiry uses saved credentials (one attempt).</p>
    </form>

    <form v-else class="login-form" @submit.prevent="submitBootstrap">
      <div class="login-brand">
        <img class="login-brand-logo" src="/logo-ruto.svg" alt="Ruto logo" />
        <h1>First Admin Setup</h1>
      </div>
      <label class="field">
        <span>Name</span>
        <input v-model="name" autocomplete="name" required />
      </label>
      <label class="field">
        <span>Username</span>
        <input v-model="username" autocomplete="username" required />
      </label>
      <label class="field">
        <span>Password</span>
        <input v-model="password" type="password" autocomplete="new-password" required />
      </label>
      <button :disabled="bootstrapLoading" class="primary-button" type="submit">
        {{ bootstrapLoading ? "Creating..." : "Create admin" }}
      </button>
      <p v-if="errorMessage" class="error">{{ errorMessage }}</p>
      <p v-if="successMessage" class="success">{{ successMessage }}</p>
      <p class="hint">Доступно только при первой установке, пока в системе нет пользователей.</p>
    </form>
  </main>
</template>
