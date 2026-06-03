<script setup lang="ts">
import { onMounted, onUnmounted } from "vue";
import { useRouter } from "vue-router";
import { darkTheme, type GlobalThemeOverrides } from "naive-ui";
import { useAuthStore } from "./stores/auth";
import NaiveMessageBridge from "./components/NaiveMessageBridge.vue";

const router = useRouter();
const authStore = useAuthStore();
const themeOverrides: GlobalThemeOverrides = {
  common: {
    primaryColor: "#3f7ee8",
    primaryColorHover: "#5d94ef",
    primaryColorPressed: "#2f65bf",
    borderRadius: "6px",
    borderRadiusSmall: "4px",
    bodyColor: "#0f1726",
    cardColor: "#172338",
    modalColor: "#172338",
    popoverColor: "#172338"
  }
};

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
  <n-config-provider :theme="darkTheme" :theme-overrides="themeOverrides">
    <n-dialog-provider>
      <n-message-provider placement="top-right">
        <NaiveMessageBridge />
        <router-view />
      </n-message-provider>
    </n-dialog-provider>
  </n-config-provider>
</template>
