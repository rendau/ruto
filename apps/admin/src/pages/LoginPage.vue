<script setup lang="ts">
import { ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { ApiError } from "../lib/api";
import { useAuthStore } from "../stores/auth";

const authStore = useAuthStore();
const route = useRoute();
const router = useRouter();

const username = ref("");
const password = ref("");
const errorMessage = ref("");

async function submit() {
  errorMessage.value = "";
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
</script>

<template>
  <main class="login-page">
    <form class="login-form" @submit.prevent="submit">
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
      <p class="hint">Token renewal on expiry uses saved credentials (one attempt).</p>
    </form>
  </main>
</template>
