<script setup lang="ts">
import { onMounted, onUnmounted } from "vue";
import { useRouter } from "vue-router";
import { useAuthStore } from "./stores/auth";
import AppToasts from "./components/AppToasts.vue";

const router = useRouter();
const authStore = useAuthStore();

function onAuthRequired() {
  authStore.logout();
  router.push({ name: "login" });
}

onMounted(() => {
  window.addEventListener("auth:required", onAuthRequired);
});

onUnmounted(() => {
  window.removeEventListener("auth:required", onAuthRequired);
});
</script>

<template>
  <router-view />
  <AppToasts />
</template>
